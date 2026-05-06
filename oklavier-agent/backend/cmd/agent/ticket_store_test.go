package main

import (
	"sync"
	"testing"
	"time"
)

func TestTicketStore_AdmitValidate(t *testing.T) {
	s := newTicketStore()
	s.Admit("bearer-1", "session-A", "user-A", "user", time.Minute)

	uid, role, ok := s.Validate("bearer-1", "session-A")
	if !ok {
		t.Fatal("Validate(legit) returned !ok")
	}
	if uid != "user-A" || role != "user" {
		t.Fatalf("uid/role mismatch: %q %q", uid, role)
	}
}

func TestTicketStore_WrongScope(t *testing.T) {
	s := newTicketStore()
	s.Admit("bearer-1", "session-A", "user-A", "user", time.Minute)
	if _, _, ok := s.Validate("bearer-1", "session-B"); ok {
		t.Fatal("scope mismatch should not validate")
	}
}

func TestTicketStore_UnknownBearer(t *testing.T) {
	s := newTicketStore()
	if _, _, ok := s.Validate("forged", "session-A"); ok {
		t.Fatal("forged bearer should not validate")
	}
}

func TestTicketStore_EmptyBearer(t *testing.T) {
	s := newTicketStore()
	if _, _, ok := s.Validate("", "session-A"); ok {
		t.Fatal("empty bearer should not validate")
	}
}

func TestTicketStore_Expired(t *testing.T) {
	s := newTicketStore()
	s.Admit("bearer-1", "session-A", "user-A", "user", 10*time.Millisecond)
	time.Sleep(30 * time.Millisecond)
	if _, _, ok := s.Validate("bearer-1", "session-A"); ok {
		t.Fatal("expired bearer should not validate")
	}
}

func TestTicketStore_Revoke(t *testing.T) {
	s := newTicketStore()
	s.Admit("bearer-1", "session-A", "user-A", "user", time.Minute)
	s.Admit("bearer-2", "session-A", "user-A", "user", time.Minute)
	s.Admit("bearer-3", "session-B", "user-B", "user", time.Minute)
	s.Revoke("session-A")

	if _, _, ok := s.Validate("bearer-1", "session-A"); ok {
		t.Fatal("bearer-1 should be revoked")
	}
	if _, _, ok := s.Validate("bearer-2", "session-A"); ok {
		t.Fatal("bearer-2 should be revoked")
	}
	if _, _, ok := s.Validate("bearer-3", "session-B"); !ok {
		t.Fatal("bearer-3 (other session) must NOT be revoked")
	}
}

func TestTicketStore_TTLClamp(t *testing.T) {
	s := newTicketStore()
	// negative + huge → clamped to 30min
	s.Admit("b1", "s", "u", "r", -1*time.Hour)
	if v, ok := s.tickets["b1"]; !ok {
		t.Fatal("admit dropped the bearer")
	} else if v.expiresAt.Sub(time.Now()) < 25*time.Minute {
		t.Fatalf("negative TTL not clamped: %v", v.expiresAt)
	}

	s.Admit("b2", "s", "u", "r", 24*time.Hour)
	if v, ok := s.tickets["b2"]; !ok {
		t.Fatal("admit dropped the bearer")
	} else if v.expiresAt.Sub(time.Now()) > 31*time.Minute {
		t.Fatalf("huge TTL not clamped: %v", v.expiresAt)
	}
}

func TestTicketStore_ConcurrentAdmitValidate(t *testing.T) {
	s := newTicketStore()
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s.Admit("b-shared", "s", "u", "r", time.Minute)
			s.Validate("b-shared", "s")
		}()
	}
	wg.Wait()
}
