/**
 * Home shell for the operator landing page. B04 is a skeleton: the page
 * renders the API health state client-side only as a smoke signal. Real
 * information architecture arrives with B09 (Slice 1 frontend workflow).
 */
export default function Home() {
  return (
    <main style={{ fontFamily: "system-ui, sans-serif", padding: "2rem" }}>
      <h1>Arrival Ready ｜迎客验收</h1>
      <p>
        工程基线已建立（B04）。业务页面从执行方案 B09 开始交付； 当前仅验证 Web ↔ Go API ↔ AI
        Service 三端连通。
      </p>
    </main>
  );
}
