import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    const apiUrl = process.env.NEXT_PUBLIC_OKLAVIER_API || "http://oklavier-api:8080";
    const oklavierApi = process.env.OKLAVIER_API || "http://oklavier-api:8080";
    return [
      {
        source: "/api/auth/oidc/:path*",
        destination: `${oklavierApi}/api/auth/oidc/:path*`,
      },
      {
        source: "/api/vnc-proxy/:path*",
        destination: `${apiUrl}/vnc/:path*`,
      },
      {
        source: "/api/vnc-ws/:path*",
        destination: `${apiUrl}/vnc-ws/:path*`,
      },
      {
        source: "/api/agent/:path*",
        destination: `${apiUrl}/api/agent/:path*`,
      },
      {
        source: "/proxy/ws/:path*",
        destination: `${oklavierApi}/proxy/ws/:path*`,
      },
    ];
  },
};

export default nextConfig;
