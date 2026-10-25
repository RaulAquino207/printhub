# PrintHub web

Next.js (App Router) frontend for PrintHub, built with TypeScript, Tailwind CSS, shadcn/ui (Base UI primitives, Vega style, preset `b1Z5aAiW0`) and Lucide icons.

Follow the Minimal / Modern design system summarized in the [frontend section of AGENTS.md](../../AGENTS.md#frontend) for all UI work.

```bash
moon run web:dev                    # http://localhost:3000
moon run web:typecheck web:lint web:format web:build
pnpm dlx shadcn@latest add <name>   # add a shadcn/ui component (run inside apps/web)
```
