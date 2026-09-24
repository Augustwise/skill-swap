import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Proxy the Go API through the frontend origin
  async rewrites() {
    return [
      {
        source: "/api/v1/:path*",
        destination: "http://127.0.0.1:8080/api/v1/:path*",
      },
    ];
  },
};

export default nextConfig;
