# Next.js App Router basics

How the PrintHub web app is organized with the App Router, and when code runs on the server or in the browser.

## 1. The problem

The web needs shared chrome (header, navigation) around several pages, a way to highlight the current page, and a way to talk to the API from the browser. With React alone you would pick a router, decide what renders where, and wire everything by hand.

## 2. The theory

- **File-system routing**: folders inside `app/` are URL segments; a `page.tsx` makes a segment reachable. `app/page.tsx` is `/`, `app/fila/page.tsx` is `/fila`.
- **Layouts**: `layout.tsx` wraps every page below it and is preserved across navigations (it does not re-render or lose state). The root layout must render `<html>` and `<body>`.
- **Server Components (default)**: components render on the server; their code is not sent to the browser. They can be `async` and read server-only data, but cannot use state, effects or browser APIs.
- **Client Components (`"use client"`)**: the directive marks a boundary; that module and everything it imports also ship to the browser and hydrate there, so they can use `useState`, `useEffect`, event handlers and browser APIs. Keep the boundary as low in the tree as possible.
- **Static rendering**: pages without request-time data are prerendered at build time (`○ (Static)` in `next build` output).
- **`NEXT_PUBLIC_*` env vars** are inlined into the browser bundle at build time; other env vars stay on the server.
- **Metadata**: `export const metadata` in a layout or page sets `<title>` and friends; `title.template` composes titles across levels.

## 3. The chosen solution

```text
apps/web/app/
├── layout.tsx        Server — html/body, fonts, theme, SiteHeader, <main>
├── page.tsx          Server — "Nova impressão"  (/)
└── fila/page.tsx     Server — "Fila de impressão" (/fila)
apps/web/components/
├── site-header.tsx   Server — brand + nav + API status, responsive layout
├── main-nav.tsx      Client — needs usePathname() for the active link
├── api-status.tsx    Client — polls the API with useEffect
├── page-header.tsx   Server — title + description
└── empty-state.tsx   Server — icon + message
```

Only the two components that need browser features are Client Components; everything else renders on the server and ships no JavaScript.

## 4. Example from our project

`main-nav.tsx` is a Client Component because it reads the current URL:

```tsx
"use client"
const pathname = usePathname()
const active = pathname === item.href
<Link href={item.href} aria-current={active ? "page" : undefined}>
```

`aria-current="page"` tells assistive technology which link is the current page; the visual underline follows the same condition.

`api-status.tsx` fetches from the browser on purpose: the request is cross-origin, so it exercises CORS ([cors.md](../http/cors.md)). A Server Component could call the API server-to-server, where CORS does not apply.

```tsx
useEffect(() => {
  const controller = new AbortController()
  // check() runs now and every 10 s
  return () => { controller.abort(); clearInterval(timer) }  // cleanup on unmount
}, [])
```

The cleanup function aborts an in-flight request and stops the timer when the component unmounts — the browser-side counterpart of Go's `context` cancellation.

Responsive header without duplicating markup: on mobile the nav gets `order-last w-full` (its own row); from `sm:` it returns to the single row.

## 5. Alternatives and trade-offs

| Alternative | Trade-off |
|---|---|
| Pages Router | Older model; no Server Components or nested layouts |
| Fetch API status in a Server Component | No CORS involved and no client JS, but the status would only refresh on navigation |
| Vite + React Router SPA | Simpler mental model, but Next.js is an explicit learning choice |

## 6. How to validate

```bash
moon run web:dev                 # http://localhost:3000, Fast Refresh on save
moon run web:build               # "/" and "/fila" listed as ○ (Static)
# Click between "Nova impressão" and "Fila de impressão": the header does not re-render
# Browser DevTools → Elements: the active link has aria-current="page"
```
