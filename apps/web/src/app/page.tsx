export default function HomePage() {
  return (
    <main className="min-h-screen bg-slate-50 p-10 text-slate-950">
      <section className="mx-auto max-w-5xl rounded-2xl border border-slate-200 bg-white p-10 shadow-sm">
        <p className="text-sm font-medium text-indigo-600">S00_HOME</p>
        <h1 className="mt-3 text-3xl font-semibold">AI Content Factory</h1>
        <p className="mt-3 max-w-2xl text-slate-600">
          P0 engineering scaffold is running. Product pages will be implemented by vertical iteration.
        </p>

        <dl className="mt-8 grid gap-4 sm:grid-cols-3">
          <div className="rounded-xl bg-slate-100 p-4">
            <dt className="text-sm text-slate-500">Content pack</dt>
            <dd className="mt-1 font-medium">novel</dd>
          </div>
          <div className="rounded-xl bg-slate-100 p-4">
            <dt className="text-sm text-slate-500">Workflow provider</dt>
            <dd className="mt-1 font-medium">mock</dd>
          </div>
          <div className="rounded-xl bg-slate-100 p-4">
            <dt className="text-sm text-slate-500">P0 status</dt>
            <dd className="mt-1 font-medium">scaffold initialized</dd>
          </div>
        </dl>
      </section>
    </main>
  );
}