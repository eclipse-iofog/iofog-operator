<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>${PRODUCT_NAME} Operator — Helm repository</title>
  <style>
    :root {
      color-scheme: light dark;
      --text: #1a1a1a;
      --muted: #555;
      --border: #ddd;
      --code-bg: #f4f4f5;
      --link: #0969da;
    }
    @media (prefers-color-scheme: dark) {
      :root {
        --text: #e6edf3;
        --muted: #8b949e;
        --border: #30363d;
        --code-bg: #161b22;
        --link: #58a6ff;
      }
    }
    body {
      font-family: system-ui, -apple-system, Segoe UI, Roboto, Helvetica, Arial, sans-serif;
      line-height: 1.6;
      max-width: 52rem;
      margin: 2rem auto;
      padding: 0 1.25rem;
      color: var(--text);
    }
    h1 { font-size: 1.75rem; line-height: 1.25; }
    h2 { font-size: 1.15rem; margin-top: 2rem; }
    p, li { color: var(--muted); }
    a { color: var(--link); }
    code, pre {
      font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace;
      font-size: 0.9em;
    }
    pre {
      background: var(--code-bg);
      border: 1px solid var(--border);
      border-radius: 6px;
      padding: 1rem;
      overflow-x: auto;
      color: var(--text);
    }
    .meta {
      font-size: 0.95rem;
      margin-bottom: 1.5rem;
    }
    .note {
      border-left: 3px solid var(--border);
      padding-left: 1rem;
      margin: 1.5rem 0;
    }
  </style>
</head>
<body>
  <h1>${PRODUCT_NAME} Operator — Helm repository</h1>
  <p class="meta">
    This page is the human-friendly landing site for the <strong>${PRODUCT_NAME}</strong> Helm chart index.
    Charts are pre-configured for <code>${CRD_GROUP}/v3</code> and images from <code>${IMAGE_REGISTRY}</code>.
  </p>

  <h2>Quick install</h2>
  <pre>helm repo add iofog-operator ${HELM_REPO_BASE_URL}
helm repo update
helm install pot iofog-operator/iofog-operator \
  --namespace iofog-system --create-namespace \
  --version ${VERSION} \
  --set controlplane.spec.auth.bootstrap.password='ReplaceMe1!'</pre>

  <p>Bootstrap password rules when <code>auth.mode=embedded</code>: at least 12 characters, one uppercase letter, and one special character. Or use <code>passwordSecretRef</code> instead of <code>--set</code>.</p>

  <h2>Verify</h2>
  <pre>kubectl get pods -n iofog-system
kubectl get controlplanes.${CRD_GROUP} -n iofog-system</pre>

  <h2>External auth example</h2>
  <pre>helm upgrade pot iofog-operator/iofog-operator \
  --namespace iofog-system \
  --version ${VERSION} \
  --set controlplane.spec.auth.mode=external \
  --set controlplane.spec.auth.issuerUrl=https://auth.example.com/realms/myrealm \
  --set controlplane.spec.auth.client.id=controller \
  --set controlplane.spec.auth.client.secret='...'</pre>

  <div class="note">
    <p><strong>Greenfield v3.8.</strong> There is no in-place upgrade from v3.7. Uninstall the legacy operator and CRDs before installing v3.8.</p>
  </div>

  <h2>More documentation</h2>
  <ul>
    <li><a href="${CHART_README_URL}">Chart README</a> — values overview and configuration notes</li>
    <li><a href="${OCI_SOURCE_REPO}">Operator repository</a> — manifests, OLM, and release artifacts</li>
    <li><a href="${OCI_SOURCE_REPO}/releases/tag/v${VERSION}">Release v${VERSION}</a> — downloadable chart tarball and release notes</li>
  </ul>

  <h2>Other mirror</h2>
  <p>
    Using <strong>${SIBLING_PRODUCT}</strong> instead?
    Add repo <code>${SIBLING_HELM_URL}</code> — same chart name, different CRD group and registry.
  </p>
</body>
</html>
