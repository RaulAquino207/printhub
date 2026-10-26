import type { NextConfig } from "next"

const nextConfig: NextConfig = {
  // The dev server only serves its scripts to localhost. Opening it from
  // another device (e.g. http://192.168.x.y:3000) needs that hostname listed,
  // or the page never hydrates. Development only; production ignores it.
  allowedDevOrigins: (process.env.PRINTHUB_WEB_DEV_HOSTS ?? "")
    .split(",")
    .map((host) => host.trim())
    .filter(Boolean),
}

export default nextConfig
