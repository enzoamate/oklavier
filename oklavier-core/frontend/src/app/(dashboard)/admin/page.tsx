"use client";

import { useTranslation } from "@/lib/i18n";
import { useAPI, usePostAPI } from "@/lib/api";
import { Users, Play, Server, Image, LayoutDashboard, Loader2, Clock, Activity, TrendingUp } from "lucide-react";

interface AuditEntry {
  id: string;
  user_id: string;
  user_email: string;
  action: string;
  resource_type: string;
  resource_id: string;
  details: string;
  ip_address: string;
  created_at: string;
}

interface AnalyticsData {
  sessions_per_day: { date: string; count: number }[];
  top_workspaces: { name: string; count: number }[];
  peak_hours: { hour: number; count: number }[];
  avg_duration_minutes: number;
  active_users_week: number;
  total_sessions_month: number;
}

const actionColors: Record<string, string> = {
  create: "bg-emerald-500/20 text-emerald-400",
  update: "bg-blue-500/20 text-blue-400",
  delete: "bg-red-500/20 text-red-400",
  destroy: "bg-red-500/20 text-red-400",
  toggle: "bg-yellow-500/20 text-yellow-400",
};

function MiniStatCard({ icon: Icon, label, value, color }: { icon: any; label: string; value: string | number; color: string }) {
  return (
    <div className="bg-white/5 border border-white/10 rounded-xl p-4">
      <div className="flex items-center gap-3">
        <div className={`p-2 rounded-lg bg-white/5 ${color}`}>
          <Icon className="size-4" />
        </div>
        <div className="min-w-0">
          <p className="text-xl font-bold text-white truncate">{value}</p>
          <p className="text-white/40 text-xs truncate">{label}</p>
        </div>
      </div>
    </div>
  );
}

function ChartCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="bg-white/5 border border-white/10 rounded-xl overflow-hidden">
      <div className="px-5 py-3 border-b border-white/10">
        <h3 className="text-white text-sm font-medium">{title}</h3>
      </div>
      <div className="p-5">{children}</div>
    </div>
  );
}

function SimpleLineChart({ data, noDataText }: { data: { date: string; count: number }[]; noDataText: string }) {
  if (!data || data.length === 0) {
    return <div className="h-[200px] flex items-center justify-center text-white/30 text-sm">{noDataText}</div>;
  }

  const maxCount = Math.max(...data.map((d) => d.count), 1);
  const w = 600;
  const h = 200;
  const padX = 40;
  const padY = 20;
  const chartW = w - padX * 2;
  const chartH = h - padY * 2;

  const points = data.map((d, i) => {
    const x = padX + (data.length === 1 ? chartW / 2 : (i / (data.length - 1)) * chartW);
    const y = padY + chartH - (d.count / maxCount) * chartH;
    return { x, y, ...d };
  });

  const polyline = points.map((p) => `${p.x},${p.y}`).join(" ");
  const areaPath = `M ${points[0].x},${padY + chartH} ${points.map((p) => `L ${p.x},${p.y}`).join(" ")} L ${points[points.length - 1].x},${padY + chartH} Z`;

  // Y-axis labels
  const yLabels = [0, Math.round(maxCount / 2), maxCount];

  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-[200px]" preserveAspectRatio="xMidYMid meet">
      <defs>
        <linearGradient id="lineGradient" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="#7096ff" stopOpacity="0.3" />
          <stop offset="100%" stopColor="#7096ff" stopOpacity="0" />
        </linearGradient>
      </defs>
      {/* Grid lines */}
      {yLabels.map((v, i) => {
        const y = padY + chartH - (v / maxCount) * chartH;
        return (
          <g key={i}>
            <line x1={padX} y1={y} x2={w - padX} y2={y} stroke="white" strokeOpacity="0.06" />
            <text x={padX - 8} y={y + 4} textAnchor="end" fill="white" fillOpacity="0.3" fontSize="10">
              {v}
            </text>
          </g>
        );
      })}
      {/* Area fill */}
      <path d={areaPath} fill="url(#lineGradient)" />
      {/* Line */}
      <polyline points={polyline} fill="none" stroke="#7096ff" strokeWidth="2" strokeLinejoin="round" />
      {/* Dots */}
      {points.map((p, i) => (
        <circle key={i} cx={p.x} cy={p.y} r="3" fill="#7096ff" stroke="#1a1f36" strokeWidth="1.5" />
      ))}
      {/* X-axis labels (show a few) */}
      {points
        .filter((_, i) => i === 0 || i === points.length - 1 || i === Math.floor(points.length / 2))
        .map((p, i) => (
          <text key={i} x={p.x} y={h - 2} textAnchor="middle" fill="white" fillOpacity="0.3" fontSize="9">
            {p.date.slice(5)}
          </text>
        ))}
    </svg>
  );
}

function SimpleBarChart({
  data,
  labelKey,
  valueKey,
  color,
  noDataText,
}: {
  data: any[];
  labelKey: string;
  valueKey: string;
  color: string;
  noDataText: string;
}) {
  if (!data || data.length === 0) {
    return <div className="h-[200px] flex items-center justify-center text-white/30 text-sm">{noDataText}</div>;
  }

  const maxVal = Math.max(...data.map((d) => d[valueKey]), 1);

  return (
    <div className="space-y-2">
      {data.map((item, i) => {
        const pct = (item[valueKey] / maxVal) * 100;
        return (
          <div key={i} className="flex items-center gap-3">
            <span className="text-white/50 text-xs w-20 truncate text-right flex-shrink-0">
              {item[labelKey]}
            </span>
            <div className="flex-1 h-6 bg-white/5 rounded overflow-hidden relative">
              <div
                className="h-full rounded transition-all duration-500"
                style={{ width: `${pct}%`, backgroundColor: color }}
              />
              <span className="absolute right-2 top-1/2 -translate-y-1/2 text-white/70 text-xs font-medium">
                {item[valueKey]}
              </span>
            </div>
          </div>
        );
      })}
    </div>
  );
}

function PeakHoursChart({ data, noDataText }: { data: { hour: number; count: number }[]; noDataText: string }) {
  if (!data || data.length === 0) {
    return <div className="h-[200px] flex items-center justify-center text-white/30 text-sm">{noDataText}</div>;
  }

  // Fill all 24 hours
  const allHours = Array.from({ length: 24 }, (_, h) => {
    const found = data.find((d) => d.hour === h);
    return { hour: h, count: found?.count ?? 0 };
  });

  const maxCount = Math.max(...allHours.map((d) => d.count), 1);
  const w = 600;
  const h = 180;
  const padX = 30;
  const padY = 15;
  const chartW = w - padX * 2;
  const chartH = h - padY * 2;
  const barW = chartW / 24 - 2;

  return (
    <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-[180px]" preserveAspectRatio="xMidYMid meet">
      {allHours.map((d, i) => {
        const barH = (d.count / maxCount) * chartH;
        const x = padX + (i / 24) * chartW + 1;
        const y = padY + chartH - barH;
        return (
          <g key={i}>
            <rect x={x} y={y} width={barW} height={barH} rx="2" fill="#65d5c5" fillOpacity={d.count > 0 ? 0.7 : 0.1} />
            {i % 3 === 0 && (
              <text x={x + barW / 2} y={h - 2} textAnchor="middle" fill="white" fillOpacity="0.3" fontSize="9">
                {d.hour}h
              </text>
            )}
          </g>
        );
      })}
    </svg>
  );
}

export default function AdminDashboard() {
  const { t } = useTranslation();

  const { data: usersData, isLoading: usersLoading } = useAPI<{ total: number }>("/api/admin/users?per_page=1");
  const { data: sessionsData, isLoading: sessionsLoading } = useAPI<{ total: number }>("/api/admin/sessions?per_page=1");
  const { data: agentsData, isLoading: agentsLoading } = useAPI<{ total: number }>("/api/admin/agents?per_page=1");
  const { data: workspacesData, isLoading: workspacesLoading } = useAPI<{ total: number }>("/api/admin/workspaces?per_page=1");
  const { data: auditData, isLoading: auditLoading } = useAPI<{ entries: AuditEntry[]; total: number }>("/api/admin/audit?page=1&per_page=5");
  const { data: analytics, isLoading: analyticsLoading } = usePostAPI<AnalyticsData>("/api/admin/analytics");

  const usersTotal = usersData?.total ?? 0;
  const sessionsTotal = sessionsData?.total ?? 0;
  const agentsTotal = agentsData?.total ?? 0;
  const workspacesTotal = workspacesData?.total ?? 0;
  const auditEntries = auditData?.entries ?? [];

  const isLoading = usersLoading && sessionsLoading && agentsLoading && workspacesLoading;

  if (isLoading) {
    return (
      <div className="flex items-center gap-2 text-white/60">
        <Loader2 className="size-5 animate-spin" /> {t("common.loading")}
      </div>
    );
  }

  const avgDuration = analytics?.avg_duration_minutes ?? 0;
  const avgDurationDisplay = avgDuration > 60
    ? `${Math.floor(avgDuration / 60)}h ${Math.round(avgDuration % 60)}${t("admin.dashboard.minutes")}`
    : `${Math.round(avgDuration)} ${t("admin.dashboard.minutes")}`;

  const noData = t("admin.dashboard.no_data");

  return (
    <div>
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-xl font-bold text-white flex items-center gap-2">
          <LayoutDashboard className="size-5 text-oklavier-blue" />
          {t("admin.dashboard.title")}
        </h1>
        <p className="text-white/40 text-sm">{t("admin.dashboard.subtitle")}</p>
      </div>

      {/* Stats cards — 6 compact cards */}
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 mb-6">
        <MiniStatCard icon={Users} label={t("admin.users.title")} value={usersTotal} color="text-blue-400" />
        <MiniStatCard icon={Play} label={t("admin.sessions.title")} value={sessionsTotal} color="text-green-400" />
        <MiniStatCard icon={Server} label={t("admin.agents.title")} value={agentsTotal} color="text-purple-400" />
        <MiniStatCard icon={Image} label={t("admin.workspaces.title")} value={workspacesTotal} color="text-orange-400" />
        <MiniStatCard icon={Activity} label={t("admin.dashboard.active_users_week")} value={analytics?.active_users_week ?? 0} color="text-teal-400" />
        <MiniStatCard icon={Clock} label={t("admin.dashboard.avg_duration")} value={analyticsLoading ? "..." : avgDurationDisplay} color="text-indigo-400" />
      </div>

      {/* Charts row */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
        <ChartCard title={t("admin.dashboard.sessions_over_time")}>
          {analyticsLoading ? (
            <div className="h-[200px] flex items-center justify-center text-white/40">
              <Loader2 className="size-4 animate-spin mr-2" /> {t("common.loading")}
            </div>
          ) : (
            <SimpleLineChart data={analytics?.sessions_per_day ?? []} noDataText={noData} />
          )}
        </ChartCard>

        <ChartCard title={t("admin.dashboard.peak_hours")}>
          {analyticsLoading ? (
            <div className="h-[180px] flex items-center justify-center text-white/40">
              <Loader2 className="size-4 animate-spin mr-2" /> {t("common.loading")}
            </div>
          ) : (
            <PeakHoursChart data={analytics?.peak_hours ?? []} noDataText={noData} />
          )}
        </ChartCard>
      </div>

      {/* Top workspaces */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
        <ChartCard title={t("admin.dashboard.top_workspaces")}>
          {analyticsLoading ? (
            <div className="h-[200px] flex items-center justify-center text-white/40">
              <Loader2 className="size-4 animate-spin mr-2" /> {t("common.loading")}
            </div>
          ) : (
            <SimpleBarChart
              data={analytics?.top_workspaces ?? []}
              labelKey="name"
              valueKey="count"
              color="#7096ff"
              noDataText={noData}
            />
          )}
        </ChartCard>

        <ChartCard title={t("admin.dashboard.total_sessions_month")}>
          <div className="h-[200px] flex flex-col items-center justify-center">
            <div className="flex items-center gap-2 mb-2">
              <TrendingUp className="size-8 text-oklavier-blue" />
              <span className="text-5xl font-bold text-white">{analytics?.total_sessions_month ?? 0}</span>
            </div>
            <p className="text-white/40 text-sm">{t("admin.dashboard.total_sessions_month")}</p>
          </div>
        </ChartCard>
      </div>

      {/* Recent activity */}
      <div className="bg-white/5 border border-white/10 rounded-xl overflow-hidden">
        <div className="px-5 py-4 border-b border-white/10">
          <h2 className="text-white font-medium">{t("admin.dashboard.recent_activity")}</h2>
        </div>
        {auditLoading && !auditData ? (
          <div className="p-5 flex items-center gap-2 text-white/60">
            <Loader2 className="size-4 animate-spin" /> {t("common.loading")}
          </div>
        ) : auditEntries.length === 0 ? (
          <div className="p-5 text-white/40 text-sm">{t("admin.audit.no_entries")}</div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="text-left text-white/40 text-xs border-b border-white/5">
                <th className="px-5 py-2 font-medium">{t("admin.audit.date")}</th>
                <th className="px-5 py-2 font-medium">{t("admin.audit.user")}</th>
                <th className="px-5 py-2 font-medium">{t("admin.audit.action")}</th>
                <th className="px-5 py-2 font-medium">{t("admin.audit.resource")}</th>
                <th className="px-5 py-2 font-medium">{t("admin.audit.details")}</th>
              </tr>
            </thead>
            <tbody>
              {auditEntries.map((entry) => (
                <tr key={entry.id} className="border-b border-white/5 last:border-0">
                  <td className="px-5 py-3 text-white/60 text-xs whitespace-nowrap">
                    {new Date(entry.created_at).toLocaleString()}
                  </td>
                  <td className="px-5 py-3 text-white/80 text-xs">
                    {entry.user_email || entry.user_id || "-"}
                  </td>
                  <td className="px-5 py-3">
                    <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${actionColors[entry.action] || "bg-white/10 text-white/60"}`}>
                      {entry.action}
                    </span>
                  </td>
                  <td className="px-5 py-3">
                    <span className="text-white/60 text-xs">{entry.resource_type}</span>
                    {entry.resource_id && (
                      <span className="text-white/30 text-xs ml-1">#{entry.resource_id.slice(0, 8)}</span>
                    )}
                  </td>
                  <td className="px-5 py-3 text-white/50 text-xs max-w-[200px] truncate">
                    {entry.details || "-"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
