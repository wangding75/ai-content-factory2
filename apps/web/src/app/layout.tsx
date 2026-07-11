import type { Metadata } from "next";
import "./globals.css";

export const metadata: Metadata = { title: "AI Content Factory", description: "Novel project workspace" };
export default function RootLayout({ children }: Readonly<{ children: React.ReactNode }>) { return <html lang="zh-CN"><body>{children}</body></html>; }