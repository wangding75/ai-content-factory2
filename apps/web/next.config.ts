import type { NextConfig } from "next";

const apiBaseUrl = process.env.API_BASE_URL ?? "http://localhost:18080/api/v1";
const nextConfig: NextConfig = {
  allowedDevOrigins: ["127.0.0.1"],
  async rewrites() {
    return [{ source: "/api/v1/:path*", destination: `${apiBaseUrl}/:path*` }];
  },
};

export default nextConfig;
