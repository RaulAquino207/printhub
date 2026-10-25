# PrintHub web

Next.js (App Router) frontend for PrintHub, built with TypeScript, Tailwind CSS, shadcn/ui (Base UI primitives, Vega style, preset `b1Z5aAiW0`) and Lucide icons.

Follow [`DESIGN_SYSTEM_MINIMAL_MODERN.md`](../../DESIGN_SYSTEM_MINIMAL_MODERN.md) for all UI work.

```bash
moon run web:dev                    # http://localhost:3000
moon run web:typecheck web:lint web:format web:build
pnpm dlx shadcn@latest add <name>   # add a shadcn/ui component (run inside apps/web)
```
