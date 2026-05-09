// Disable SSR: this is a pure client-side SPA served via adapter-static.
// All auth and WS logic runs in the browser only.
export const prerender = true;
export const ssr = false;
