(() => {
  const canvas = document.getElementById("starfield");
  if (!canvas) return;
  const ctx = canvas.getContext("2d");

  let width = 0;
  let height = 0;
  let stars = [];
  let animId = null;
  let lastDrawTime = 0;
  const frameInterval = 1000 / 32; // Throttle to ~32fps for silky twinkle & ultra-low CPU/battery footprint
  const starCount = 140;

  function resize() {
    const dpr = Math.min(window.devicePixelRatio || 1, 2);
    width = window.innerWidth;
    height = window.innerHeight;
    canvas.width = Math.floor(width * dpr);
    canvas.height = Math.floor(height * dpr);
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    initStars();
  }

  function initStars() {
    stars = [];
    for (let i = 0; i < starCount; i++) {
      stars.push({
        x: Math.random() * width,
        y: Math.random() * height * 0.85,
        r: Math.random() * 1.3 + 0.35,
        alpha: Math.random() * 0.65 + 0.2,
        speed: Math.random() * 0.015 + 0.004,
        phase: Math.random() * Math.PI * 2,
        warm: Math.random() < 0.16,
      });
    }
  }

  function draw(t) {
    if (document.hidden) return;

    if (t - lastDrawTime < frameInterval) {
      animId = requestAnimationFrame(draw);
      return;
    }
    lastDrawTime = t;

    const grad = ctx.createLinearGradient(0, 0, 0, height);
    grad.addColorStop(0, "#04060d");
    grad.addColorStop(0.62, "#0b1124");
    grad.addColorStop(1, "#141628");
    ctx.fillStyle = grad;
    ctx.fillRect(0, 0, width, height);

    const time = t * 0.001;
    for (const s of stars) {
      const flicker = 0.65 + 0.35 * Math.sin(time * s.speed * 60 + s.phase);
      const a = Math.max(0.08, Math.min(0.95, s.alpha * flicker));
      ctx.beginPath();
      ctx.arc(s.x, s.y, s.r, 0, Math.PI * 2);
      ctx.fillStyle = s.warm
        ? `rgba(242, 184, 128, ${a})`
        : `rgba(246, 242, 235, ${a})`;
      ctx.fill();
    }

    animId = requestAnimationFrame(draw);
  }

  function onVisibilityChange() {
    if (document.hidden) {
      if (animId) {
        cancelAnimationFrame(animId);
        animId = null;
      }
    } else if (!animId) {
      lastDrawTime = performance.now();
      animId = requestAnimationFrame(draw);
    }
  }

  window.addEventListener("resize", resize);
  document.addEventListener("visibilitychange", onVisibilityChange);

  resize();
  animId = requestAnimationFrame(draw);
})();
