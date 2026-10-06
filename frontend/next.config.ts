import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // Standalone output bundles only the files needed to run the app,
  // which is more reliable in Railway Docker containers.
  output: "standalone",
  turbopack: { root: process.cwd() },
};

export default nextConfig;
