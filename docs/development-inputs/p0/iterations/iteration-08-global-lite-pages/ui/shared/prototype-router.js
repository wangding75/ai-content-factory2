(() => {
  const routes = new Set(["/", "/projects", "/materials", "/works", "/workflows", "/settings"]);
  const current = window.location.pathname.replace(/\/$/, "") || "/";
  document.querySelectorAll("a[href]").forEach((link) => {
    const target = link.getAttribute("href");
    if (!target || target === "#") {
      link.setAttribute("aria-disabled", "true");
      link.removeAttribute("href");
      return;
    }
    if (routes.has(target) && target === current) {
      link.setAttribute("aria-current", "page");
    }
  });
  document.querySelectorAll("button[data-acf-disabled]").forEach((button) => {
    button.disabled = true;
    button.setAttribute("aria-disabled", "true");
  });
})();
