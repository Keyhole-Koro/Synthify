const features = [
  "Next.js 16 + React 19",
  "Tailwind CSS 4",
  "TypeScript strict mode",
  "Playwright E2E"
];

export default function HomePage() {
  return (
    <main className="min-h-screen bg-[radial-gradient(circle_at_top,_#d1fae5,_#f8fafc_45%,_#e2e8f0)] text-slate-950">
      <div className="mx-auto flex min-h-screen max-w-6xl flex-col justify-between px-6 py-10 lg:px-12">
        <header className="flex items-center justify-between">
          <span className="rounded-full border border-slate-300/80 bg-white/70 px-4 py-2 text-xs font-semibold tracking-[0.3em] text-slate-700 uppercase backdrop-blur">
            Synthify
          </span>
          <span className="text-sm text-slate-600">next starter</span>
        </header>

        <section className="grid gap-10 py-16 lg:grid-cols-[1.4fr_0.9fr] lg:items-end">
          <div className="space-y-6">
            <p className="text-sm font-medium tracking-[0.25em] text-emerald-700 uppercase">
              Frontend Workspace
            </p>
            <h1 className="max-w-3xl text-5xl font-black tracking-tight text-balance sm:text-6xl">
              Next.js, Tailwind, and Playwright are ready to ship.
            </h1>
            <p className="max-w-2xl text-lg leading-8 text-slate-700">
              The dev server stays on port{" "}
              <code className="rounded bg-slate-900 px-2 py-1 text-sm text-white">
                5173
              </code>
              , and static export output is emitted to{" "}
              <code className="rounded bg-slate-900 px-2 py-1 text-sm text-white">
                out
              </code>
              .
            </p>
          </div>

          <div className="rounded-[2rem] border border-white/70 bg-white/80 p-6 shadow-[0_20px_80px_rgba(15,23,42,0.12)] backdrop-blur">
            <p className="text-sm font-semibold text-slate-500">
              Included setup
            </p>
            <ul className="mt-4 space-y-3">
              {features.map((feature) => (
                <li
                  key={feature}
                  className="rounded-2xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm font-medium text-slate-700"
                >
                  {feature}
                </li>
              ))}
            </ul>
          </div>
        </section>
      </div>
    </main>
  );
}
