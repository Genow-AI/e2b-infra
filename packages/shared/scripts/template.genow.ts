import { Template } from "e2b";

// Sandbox template with the data/deck libraries preinstalled.
//
// Both ecosystems are anchored at the filesystem root on purpose. envd builds
// each process's environment from scratch — PATH/HOME/USER plus whatever
// POST /sandboxes passed in — and runs it under `/bin/sh -c`, never a login
// shell (packages/envd/internal/services/process/handler/handler.go). So
// /etc/profile.d, .bashrc, venv activate and template-time ENV are all invisible
// to code the SDK runs. A root-level /node_modules and a system-python install
// are what Node and Python resolve from any cwd with no environment at all;
// `npm install -g` would not resolve, and NODE_PATH does not work for ESM.
//
// Versions are pinned inline because the layer cache hashes the literal command
// string (build/phases/steps/hash.go), not the resolved versions — unpinned, a
// cache miss silently installs whatever is latest that day.
export const template = Template()
  .fromBaseImage()
  .runCmd("pip install --break-system-packages pandas==2.2.3 matplotlib==3.9.2", {
    user: "root",
  })
  .runCmd("npm install --prefix / pptxgenjs@3.12.0", { user: "root" })
  // Headless backend via config file, not MPLBACKEND: env vars cannot be baked
  // into a template, only passed per-sandbox on POST /sandboxes.
  .runCmd(
    "mkdir -p ~/.config/matplotlib && echo 'backend: Agg' > ~/.config/matplotlib/matplotlibrc",
  )
  // Warm the font cache as `user` (HOME=/home/user). Built as root it lands in
  // /root/.cache, is never read, and every sandbox re-pays the ~5-15s font scan.
  .runCmd('python3 -c "import pandas, matplotlib.pyplot"');
