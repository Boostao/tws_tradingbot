#!/usr/bin/env node
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

export function escapeHtml(value) {
  return String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');
}

export function auditZeroMarkdown(distDir) {
  if (!fs.existsSync(distDir)) {
    return [];
  }
  const violations = [];
  function scan(dir) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        scan(fullPath);
      } else if (entry.isFile()) {
        const lower = entry.name.toLowerCase();
        if (lower.endsWith('.md') || lower.endsWith('.markdown')) {
          violations.push(path.relative(distDir, fullPath));
        }
      }
    }
  }
  scan(distDir);
  return violations;
}

export function buildSite({
  siteDir = __dirname,
  distDir = path.join(__dirname, 'dist'),
} = {}) {
  const manifestPath = path.join(siteDir, 'release-manifest.json');
  const stylesPath = path.join(siteDir, 'styles.css');
  const faviconSvgPath = path.join(siteDir, 'favicon.svg');

  if (!fs.existsSync(distDir)) {
    fs.mkdirSync(distDir, { recursive: true });
  }

  // Load manifest
  let manifest = {
    version: '1.0.0',
    title: 'tws_traderbot — Poste de pilotage Cobalt',
    release_date: new Date().toISOString().split('T')[0],
    description: 'Poste de pilotage & exécution algorithmique F1 pour la méthode Cobalt Investissements',
    artifacts: [],
    release_notes: 'Production release of tws_traderbot.'
  };

  if (fs.existsSync(manifestPath)) {
    try {
      manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf8'));
    } catch (err) {
      console.warn('Warning: Could not parse release-manifest.json, using defaults.', err);
    }
  }

  // Copy CSS
  if (fs.existsSync(stylesPath)) {
    fs.copyFileSync(stylesPath, path.join(distDir, 'styles.css'));
  }

  // Copy SVG favicon
  if (fs.existsSync(faviconSvgPath)) {
    fs.copyFileSync(faviconSvgPath, path.join(distDir, 'favicon.svg'));
  }

  // Copy manifest
  fs.writeFileSync(
    path.join(distDir, 'release-manifest.json'),
    JSON.stringify(manifest, null, 2) + '\n',
    'utf8'
  );

  // Write .nojekyll
  fs.writeFileSync(path.join(distDir, '.nojekyll'), '', 'utf8');

  // Generate HTML
  const artifacts = manifest.artifacts || [];
  const artifactCardsHtml = artifacts.map(art => `
      <div class="download-card">
        <div class="card-header">
          <svg class="card-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            ${art.os_key === 'windows'
              ? '<path d="M3 5.3l7.4-1v7.1H3V5.3zm8.4-1.1L21 3v8.4h-9.6V4.2zM3 12.6h7.4v7.1l-7.4-1v-6.1zm8.4 0H21V21l-9.6-1.3v-7.1z"/>'
              : '<path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm-1 14.5v-9l6 4.5-6 4.5z"/>'}
          </svg>
          <div>
            <div class="card-platform">${escapeHtml(art.platform || art.os_key || 'Platform')}</div>
            <div class="card-arch">${escapeHtml(art.arch || 'amd64')} • ${escapeHtml(art.format || 'Archive')}</div>
          </div>
        </div>
        <div class="card-file">${escapeHtml(art.filename)} (${escapeHtml(art.size || 'unknown size')})</div>
        <a class="btn-download" href="${escapeHtml(art.filename)}">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
            <polyline points="7 10 12 15 17 10"/>
            <line x1="12" y1="15" x2="12" y2="3"/>
          </svg>
          Télécharger / Download
        </a>
      </div>
  `).join('');

  const checksumRowsHtml = artifacts.map(art => `
        <tr>
          <td><strong>${escapeHtml(art.platform || 'OS')}</strong></td>
          <td>${escapeHtml(art.arch || 'amd64')}</td>
          <td><code>${escapeHtml(art.filename)}</code></td>
          <td>${escapeHtml(art.size || '—')}</td>
          <td><span class="code-hash">${escapeHtml(art.sha256 || 'pending')}</span></td>
        </tr>
  `).join('');

  const html = `<!DOCTYPE html>
<html lang="fr">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <link rel="icon" type="image/svg+xml" href="favicon.svg">
  <link rel="stylesheet" href="styles.css">
  <title>${escapeHtml(manifest.title || 'tws_traderbot — Poste de pilotage')}</title>
</head>
<body>
  <div class="portal-container">
    <header>
      <div class="brand-row">
        <img class="brand-logo" src="favicon.svg" alt="tws_traderbot logo">
        <div>
          <h1>${escapeHtml(manifest.name || 'tws_traderbot')}</h1>
          <p class="subtitle">${escapeHtml(manifest.title || 'Poste de pilotage Cobalt')}</p>
        </div>
      </div>
      <div class="meta-badges">
        <span class="badge badge-version">v${escapeHtml(manifest.version || '1.0.0')}</span>
        <span class="badge badge-date">Date: ${escapeHtml(manifest.release_date || '2026-11-16')}</span>
        <span class="badge badge-status">Release officielle / Stable</span>
      </div>
    </header>

    <main>
      <h2 class="section-title">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4"/>
          <polyline points="7 10 12 15 17 10"/>
          <line x1="12" y1="15" x2="12" y2="3"/>
        </svg>
        Téléchargements / Packages
      </h2>
      <div class="download-grid">
        ${artifactCardsHtml}
      </div>

      <h2 class="section-title">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z"/>
          <polyline points="9 12 11 14 15 10"/>
        </svg>
        Table d'empreintes SHA-256 / Verification Hashes
      </h2>
      <div class="table-wrapper">
        <table>
          <thead>
            <tr>
              <th>Plateforme</th>
              <th>Arch</th>
              <th>Fichier</th>
              <th>Taille</th>
              <th>Empreinte SHA-256</th>
            </tr>
          </thead>
          <tbody>
            ${checksumRowsHtml}
          </tbody>
        </table>
      </div>

      <h2 class="section-title">
        <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
          <polyline points="14 2 14 8 20 8"/>
          <line x1="16" y1="13" x2="8" y2="13"/>
          <line x1="16" y1="17" x2="8" y2="17"/>
          <polyline points="10 9 9 9 8 9"/>
        </svg>
        Notes de version / Release Notes
      </h2>
      <div class="release-notes-box">
        <p>${escapeHtml(manifest.release_notes || '')}</p>
        <ul>
          <li><strong>Poste de pilotage F1</strong>: 3 espaces de travail (Achat, Short, Swing) avec créneau stratégique actif exclusif (1/1).</li>
          <li><strong>Gestionnaire de Watchlists</strong>: 5 listes pré-remplies Cobalt, normalisation SYMBOL.VENUE et classification des actifs négociables.</li>
          <li><strong>Moteur d'exécution Fail-Closed</strong>: Circuit-breaker principal et coupe-circuit automatique de perte quotidienne.</li>
          <li><strong>Catalogue Cobalt</strong>: 20 préconfigurations et modèles graphiques prêts à l'emploi.</li>
          <li><strong>Intégrité linguistique</strong>: Lexique Cobalt authentique et parité 100% français/anglais.</li>
        </ul>
      </div>
    </main>

    <footer>
      <p>tws_traderbot • Cobalt Investissements Execution Augmenter • Wails v3 &amp; Svelte 5</p>
    </footer>
  </div>
</body>
</html>
`;

  fs.writeFileSync(path.join(distDir, 'index.html'), html, 'utf8');

  // Feature 48: Zero-Markdown Allowlist Gate in site/dist/
  const markdownViolations = auditZeroMarkdown(distDir);
  if (markdownViolations.length > 0) {
    const errorMsg = `ERROR: Zero-markdown gate violation: found markdown file(s) in ${distDir}: ${markdownViolations.join(', ')}`;
    console.error(errorMsg);
    throw new Error(errorMsg);
  }

  return { distDir, artifactCount: artifacts.length };
}

// Direct execution CLI entrypoint
if (process.argv[1] && fileURLToPath(import.meta.url) === path.resolve(process.argv[1])) {
  try {
    const result = buildSite();
    console.log(`Site build successful: ${result.distDir} generated with ${result.artifactCount} artifacts (0 markdown files).`);
  } catch (err) {
    console.error('Site build failed:', err.message);
    process.exit(1);
  }
}
