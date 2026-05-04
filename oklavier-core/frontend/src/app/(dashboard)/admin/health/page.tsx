"use client";

import { Activity, Database, Server, Users, Cpu } from "lucide-react";
import { useTranslation } from "@/lib/i18n";
import { usePostAPI } from "@/lib/api";

interface AgentHealth {
  id: string;
  name: string;
  status: string;
  last_heartbeat: string;
  active_sessions: number;
}

interface HealthData {
  database: { status: string; type: string };
  valkey: { status: string; type: string };
  agents: AgentHealth[];
  stats: { active_sessions: number; total_users: number };
}

function StatusDot({ status }: { status: string }) {
  const isHealthy = status === "healthy" || status === "connected";
  return (
    <span
      className={`inline-block size-3 rounded-full ${
        isHealthy
          ? "bg-emerald-500 shadow-[0_0_8px_rgba(16,185,129,0.5)]"
          : "bg-red-500 shadow-[0_0_8px_rgba(239,68,68,0.5)]"
      }`}
    />
  );
}

function timeAgo(dateStr: string) {
  if (!dateStr) return "-";
  const diff = Date.now() - new Date(dateStr).getTime();
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  return `${hours}h ${mins % 60}m ago`;
}

export default function HealthPage() {
  const { t } = useTranslation();
  const { data } = usePostAPI<HealthData>("/api/admin/health", { refreshInterval: 10000 });

  return (
    <div>
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-white flex items-center gap-3">
            <Activity className="size-6 text-oklavier-blue" />
            {t("admin.health.title")}
          </h1>
          <p className="text-white/50 text-sm mt-1">{t("admin.health.subtitle")}</p>
        </div>
      </div>

      {/* Service status cards */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        {/* Database */}
        <div className="bg-[#1a1f36] border border-white/10 rounded-xl p-5">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Database className="size-5 text-oklavier-blue" />
              <span className="text-white font-medium">{t("admin.health.database")}</span>
            </div>
            <StatusDot status={data?.database?.status || "unhealthy"} />
          </div>
          <p className="text-white/40 text-sm">{data?.database?.type || "PostgreSQL"}</p>
          <p className="text-white/60 text-sm mt-1">
            {data?.database?.status === "healthy" ? t("admin.health.status_healthy") : t("admin.health.status_unhealthy")}
          </p>
        </div>

        {/* Valkey */}
        <div className="bg-[#1a1f36] border border-white/10 rounded-xl p-5">
          <div className="flex items-center justify-between mb-3">
            <div className="flex items-center gap-2">
              <Cpu className="size-5 text-oklavier-blue" />
              <span className="text-white font-medium">{t("admin.health.valkey")}</span>
            </div>
            <StatusDot status={data?.valkey?.status || "unhealthy"} />
          </div>
          <p className="text-white/40 text-sm">{data?.valkey?.type || "Valkey"}</p>
          <p className="text-white/60 text-sm mt-1">
            {data?.valkey?.status === "healthy" ? t("admin.health.status_healthy") : t("admin.health.status_unhealthy")}
          </p>
        </div>

        {/* Active sessions */}
        <div className="bg-[#1a1f36] border border-white/10 rounded-xl p-5">
          <div className="flex items-center gap-2 mb-3">
            <Activity className="size-5 text-oklavier-blue" />
            <span className="text-white font-medium">{t("admin.health.active_sessions")}</span>
          </div>
          <p className="text-3xl font-bold text-white">{data?.stats?.active_sessions ?? "-"}</p>
        </div>

        {/* Total users */}
        <div className="bg-[#1a1f36] border border-white/10 rounded-xl p-5">
          <div className="flex items-center gap-2 mb-3">
            <Users className="size-5 text-oklavier-blue" />
            <span className="text-white font-medium">{t("admin.health.total_users")}</span>
          </div>
          <p className="text-3xl font-bold text-white">{data?.stats?.total_users ?? "-"}</p>
        </div>
      </div>

      {/* Agents */}
      <div className="bg-[#1a1f36] border border-white/10 rounded-xl overflow-hidden">
        <div className="px-5 py-4 border-b border-white/10 flex items-center gap-2">
          <Server className="size-5 text-oklavier-blue" />
          <h2 className="text-white font-semibold">{t("admin.health.agents")}</h2>
          <span className="text-white/40 text-sm ml-2">
            {data?.agents?.length ?? 0} {t("admin.health.registered")}
          </span>
        </div>
        {(!data?.agents || data.agents.length === 0) ? (
          <div className="px-5 py-8 text-center text-white/40 text-sm">
            {t("admin.health.no_agents")}
          </div>
        ) : (
          <table className="w-full">
            <thead>
              <tr className="text-white/40 text-xs uppercase tracking-wider border-b border-white/5">
                <th className="text-left px-5 py-3">{t("admin.health.agent_name")}</th>
                <th className="text-left px-5 py-3">{t("admin.health.status")}</th>
                <th className="text-left px-5 py-3">{t("admin.health.last_heartbeat")}</th>
                <th className="text-left px-5 py-3">{t("admin.health.sessions")}</th>
              </tr>
            </thead>
            <tbody>
              {data.agents.map((agent) => (
                <tr key={agent.id} className="border-b border-white/5 hover:bg-white/5">
                  <td className="px-5 py-3 text-white text-sm font-medium">{agent.name}</td>
                  <td className="px-5 py-3">
                    <div className="flex items-center gap-2">
                      <StatusDot status={agent.status} />
                      <span className="text-white/70 text-sm capitalize">{agent.status}</span>
                    </div>
                  </td>
                  <td className="px-5 py-3 text-white/50 text-sm">{timeAgo(agent.last_heartbeat)}</td>
                  <td className="px-5 py-3 text-white/70 text-sm">{agent.active_sessions}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      <p className="text-white/20 text-xs mt-4 text-right">{t("admin.health.auto_refresh")}</p>
    </div>
  );
}
