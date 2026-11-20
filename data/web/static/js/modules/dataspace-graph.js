/**
 * RFID Dataspace - Mermaid Knowledge Graph Visualizer Module
 * Connects physical RFID/NFC tag scans to Alma records and OpenAlex scholarly graph.
 */

let currentDirection = 'TD';
let currentZoom = 1.0;
let panX = 0;
let panY = 0;
let isDragging = false;
let startX = 0;
let startY = 0;
let fetchTimeout = null;
let currentRenderId = 0;
let lastBarcodesKey = null;
let lastRenderedDirection = 'TD';
let lastMermaid = '';

function isDarkMode() {
    const root = document.documentElement;
    const mode = root.getAttribute('data-theme-mode') || root.getAttribute('data-theme') || '';
    if (mode.includes('dark') || mode.includes('black') || mode.includes('night') || mode.includes('dim')) {
        return true;
    }
    if (mode === 'auto' || !mode) {
        return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches;
    }
    return false;
}

function initMermaidConfig() {
    if (typeof window.mermaid === 'undefined') return;
    const dark = isDarkMode();
    window.mermaid.initialize({
        startOnLoad: false,
        theme: dark ? 'dark' : 'default',
        securityLevel: 'loose',
        flowchart: {
            useMaxWidth: false,
            htmlLabels: true,
            curve: 'basis'
        }
    });
}

function getActiveBarcodes() {
    const tbody = document.getElementById('loan-table-body');
    if (!tbody) return [];
    const barcodes = new Set();
    const rows = tbody.querySelectorAll('.tag-row, [data-itemid]');
    rows.forEach(row => {
        const id = row.getAttribute('data-itemid') || row.dataset.itemid;
        if (id && id.trim() && !id.includes('item.noItemId') && id !== '–') {
            barcodes.add(id.trim());
        }
    });
    return Array.from(barcodes);
}

function updateTransform(container) {
    const svg = container?.querySelector('svg');
    if (!svg) return;
    svg.style.transform = `translate(${panX}px, ${panY}px) scale(${currentZoom})`;
    svg.style.transformOrigin = 'center center';
    svg.style.transition = isDragging ? 'none' : 'transform 0.15s ease-out';
}

function resetZoomAndPan(container) {
    currentZoom = 1.0;
    panX = 0;
    panY = 0;
    updateTransform(container);
    const resetBtn = document.getElementById('graph-zoom-reset');
    if (resetBtn) resetBtn.textContent = '100%';
}

function updateCopyButtonState() {
    const copyBtn = document.getElementById('graph-copy-btn');
    if (copyBtn) {
        copyBtn.disabled = !lastMermaid || lastMermaid.trim().length === 0;
    }
}

async function copyToClipboard(text) {
    if (navigator.clipboard && window.isSecureContext) {
        return navigator.clipboard.writeText(text);
    }
    const textArea = document.createElement('textarea');
    textArea.value = text;
    textArea.style.position = 'fixed';
    textArea.style.left = '-999999px';
    textArea.style.top = '-999999px';
    document.body.appendChild(textArea);
    textArea.focus();
    textArea.select();
    return new Promise((resolve, reject) => {
        try {
            const successful = document.execCommand('copy');
            if (successful) resolve();
            else reject(new Error('execCommand copy failed'));
        } catch (err) {
            reject(err);
        } finally {
            textArea.remove();
        }
    });
}

export async function fetchAndRenderGraph(immediate = false, force = false) {
    if (fetchTimeout) {
        clearTimeout(fetchTimeout);
        fetchTimeout = null;
    }

    const execute = async () => {
        const viewport = document.getElementById('dataspace-graph-viewport');
        const output = document.getElementById('dataspace-mermaid-output');
        const emptyState = document.getElementById('dataspace-graph-empty');
        const loadingState = document.getElementById('dataspace-graph-loading');
        const badge = document.getElementById('graph-node-count-badge');

        if (!viewport || !output) return;

        const barcodes = getActiveBarcodes();
        const barcodesKey = [...barcodes].sort().join(',');

        if (barcodes.length === 0) {
            lastMermaid = '';
            updateCopyButtonState();
            if (lastBarcodesKey === '' && !force) {
                return;
            }
            lastBarcodesKey = '';
            lastRenderedDirection = currentDirection;
            output.innerHTML = '';
            if (emptyState) emptyState.classList.remove('hidden');
            if (loadingState) loadingState.classList.add('hidden');
            if (badge) {
                badge.classList.add('hidden');
                badge.textContent = '0 Nodes';
            }
            currentZoom = 1.0;
            panX = 0;
            panY = 0;
            const resetBtn = document.getElementById('graph-zoom-reset');
            if (resetBtn) resetBtn.textContent = '100%';
            return;
        }

        const hasSvg = !!output.querySelector('svg');
        // Prevent re-rendering and zooming jump if media list & layout direction haven't changed
        if (!force && barcodesKey === lastBarcodesKey && currentDirection === lastRenderedDirection && hasSvg) {
            return;
        }

        const wasEmpty = (lastBarcodesKey === '' || lastBarcodesKey === null);

        if (emptyState) emptyState.classList.add('hidden');
        if (loadingState) loadingState.classList.remove('hidden');

        const renderId = ++currentRenderId;

        try {
            initMermaidConfig();
            let path = window.location.pathname.replace(/\/(?:loan|encode|inventory|dataspace)\/?$/, '/');
            if (!path.endsWith('/')) path += '/';
            const url = new URL(path + 'dataspace/graph', window.location.origin);
            url.searchParams.set('barcodes', barcodes.join(','));
            url.searchParams.set('direction', currentDirection);

            const resp = await fetch(url.href);
            if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
            const data = await resp.json();

            if (renderId !== currentRenderId) return; // Stale response

            if (!data.mermaid || !window.mermaid) {
                throw new Error('Invalid Mermaid payload');
            }

            const graphId = `mermaid-graph-${Date.now()}`;
            const { svg } = await window.mermaid.render(graphId, data.mermaid);

            if (renderId !== currentRenderId) return;

            output.innerHTML = svg;
            const svgElement = output.querySelector('svg');
            if (svgElement) {
                svgElement.setAttribute('width', '100%');
                svgElement.style.maxWidth = 'none';
                svgElement.style.height = 'auto';
                svgElement.style.maxHeight = '75vh';
                svgElement.classList.add('select-none');
            }

            lastBarcodesKey = barcodesKey;
            lastRenderedDirection = currentDirection;
            lastMermaid = data.mermaid;
            updateCopyButtonState();

            // Preserve current zoom level and pan on reload/updates; only reset if starting from empty state
            if (wasEmpty) {
                resetZoomAndPan(output);
            } else {
                updateTransform(output);
                const resetBtn = document.getElementById('graph-zoom-reset');
                if (resetBtn) resetBtn.textContent = `${Math.round(currentZoom * 100)}%`;
            }

            if (badge) {
                badge.textContent = `${data.nodeCount} Nodes • ${data.edgeCount} Edges`;
                badge.classList.remove('hidden');
            }
        } catch (err) {
            console.warn('[Dataspace Graph] Render warning:', err);
            lastMermaid = '';
            updateCopyButtonState();
            if (renderId === currentRenderId) {
                output.innerHTML = `<div class="alert alert-warning text-xs max-w-lg shadow-sm">
                    <span>${err.message || 'Graph visualizer unavailable'}</span>
                </div>`;
            }
        } finally {
            if (renderId === currentRenderId && loadingState) {
                loadingState.classList.add('hidden');
            }
        }
    };

    if (immediate) {
        await execute();
    } else {
        fetchTimeout = setTimeout(execute, 250);
    }
}

export function initDataspaceGraph() {
    const graphSection = document.getElementById('dataspace-graph-section');
    const viewport = document.getElementById('dataspace-graph-viewport');
    const output = document.getElementById('dataspace-mermaid-output');
    if (!graphSection || !viewport || !output) return;

    initMermaidConfig();

    // Direction controls
    const dirTDBtn = document.getElementById('graph-dir-td');
    const dirLRBtn = document.getElementById('graph-dir-lr');
    if (dirTDBtn && dirLRBtn) {
        dirTDBtn.addEventListener('click', () => {
            if (currentDirection === 'TD') return;
            currentDirection = 'TD';
            dirTDBtn.className = 'btn btn-xs join-item btn-primary';
            dirLRBtn.className = 'btn btn-xs join-item btn-ghost';
            fetchAndRenderGraph(true, true);
        });

        dirLRBtn.addEventListener('click', () => {
            if (currentDirection === 'LR') return;
            currentDirection = 'LR';
            dirLRBtn.className = 'btn btn-xs join-item btn-primary';
            dirTDBtn.className = 'btn btn-xs join-item btn-ghost';
            fetchAndRenderGraph(true, true);
        });
    }

    // Zoom controls
    const zoomInBtn = document.getElementById('graph-zoom-in');
    const zoomOutBtn = document.getElementById('graph-zoom-out');
    const zoomResetBtn = document.getElementById('graph-zoom-reset');

    const applyZoom = (delta) => {
        currentZoom = Math.min(3.5, Math.max(0.25, currentZoom + delta));
        if (zoomResetBtn) zoomResetBtn.textContent = `${Math.round(currentZoom * 100)}%`;
        updateTransform(output);
    };

    zoomInBtn?.addEventListener('click', () => applyZoom(0.2));
    zoomOutBtn?.addEventListener('click', () => applyZoom(-0.2));
    zoomResetBtn?.addEventListener('click', () => resetZoomAndPan(output));

    // Mouse wheel zoom
    viewport.addEventListener('wheel', (e) => {
        if (!output.querySelector('svg')) return;
        e.preventDefault();
        const delta = e.deltaY < 0 ? 0.15 : -0.15;
        applyZoom(delta);
    }, { passive: false });

    // Pan drag interactions
    viewport.addEventListener('mousedown', (e) => {
        if (e.button !== 0 || !output.querySelector('svg')) return;
        isDragging = true;
        startX = e.clientX - panX;
        startY = e.clientY - panY;
        viewport.style.cursor = 'grabbing';
    });

    window.addEventListener('mousemove', (e) => {
        if (!isDragging) return;
        panX = e.clientX - startX;
        panY = e.clientY - startY;
        updateTransform(output);
    });

    window.addEventListener('mouseup', () => {
        if (isDragging) {
            isDragging = false;
            viewport.style.cursor = '';
        }
    });

    // Fullscreen state helpers
    const isSectionFullscreen = () => {
        return document.fullscreenElement === graphSection || graphSection.classList.contains('is-fullscreen');
    };

    const updateFullscreenUI = () => {
        const isFull = isSectionFullscreen();
        const expandIcon = document.getElementById('graph-fullscreen-expand-icon');
        const collapseIcon = document.getElementById('graph-fullscreen-collapse-icon');
        const textSpan = document.getElementById('graph-fullscreen-text');
        const btn = document.getElementById('graph-fullscreen-btn');

        if (expandIcon && collapseIcon) {
            if (isFull) {
                expandIcon.classList.add('hidden');
                collapseIcon.classList.remove('hidden');
            } else {
                expandIcon.classList.remove('hidden');
                collapseIcon.classList.add('hidden');
            }
        }

        if (textSpan && btn) {
            const expandText = btn.dataset.expandText || 'Vollbild';
            const collapseText = btn.dataset.collapseText || 'Verkleinern';
            textSpan.textContent = isFull ? collapseText : expandText;
        }

        if (isFull) {
            document.body.classList.add('overflow-hidden');
        } else {
            document.body.classList.remove('overflow-hidden');
        }

        setTimeout(() => updateTransform(output), 150);
    };

    const toggleFullscreen = async () => {
        if (isSectionFullscreen()) {
            if (document.fullscreenElement) {
                try {
                    await document.exitFullscreen();
                } catch (e) {
                    console.warn('[Dataspace Graph] Exit fullscreen error:', e);
                }
            }
            graphSection.classList.remove('is-fullscreen');
            updateFullscreenUI();
        } else {
            if (graphSection.requestFullscreen) {
                try {
                    await graphSection.requestFullscreen();
                    graphSection.classList.add('is-fullscreen');
                    updateFullscreenUI();
                    return;
                } catch (e) {
                    console.warn('[Dataspace Graph] Native fullscreen rejected, falling back to CSS overlay:', e);
                }
            }
            graphSection.classList.add('is-fullscreen');
            updateFullscreenUI();
        }
    };

    // Fullscreen toggle event
    const fullscreenBtn = document.getElementById('graph-fullscreen-btn');
    if (fullscreenBtn) {
        fullscreenBtn.addEventListener('click', (e) => {
            e.preventDefault();
            toggleFullscreen();
        });
    }

    document.addEventListener('fullscreenchange', () => {
        if (!document.fullscreenElement && graphSection.classList.contains('is-fullscreen')) {
            graphSection.classList.remove('is-fullscreen');
        } else if (document.fullscreenElement === graphSection) {
            graphSection.classList.add('is-fullscreen');
        }
        updateFullscreenUI();
    });

    window.addEventListener('keydown', (e) => {
        if (e.key === 'Escape' && graphSection.classList.contains('is-fullscreen')) {
            if (document.fullscreenElement) {
                document.exitFullscreen().catch(() => {});
            }
            graphSection.classList.remove('is-fullscreen');
            updateFullscreenUI();
        }
    });

    // Refresh button
    document.getElementById('graph-refresh-btn')?.addEventListener('click', () => {
        fetchAndRenderGraph(true, true);
    });

    // Copy Mermaid Button
    const copyBtn = document.getElementById('graph-copy-btn');
    const copyIcon = document.getElementById('graph-copy-icon');
    const copiedIcon = document.getElementById('graph-copied-icon');
    const copyText = document.getElementById('graph-copy-text');
    let copyTimeout = null;

    if (copyBtn) {
        copyBtn.addEventListener('click', async (e) => {
            e.preventDefault();
            if (!lastMermaid) return;
            try {
                await copyToClipboard(lastMermaid);
                if (copyIcon) copyIcon.classList.add('hidden');
                if (copiedIcon) copiedIcon.classList.remove('hidden');
                const copiedLabel = copyBtn.getAttribute('data-copied-text') || 'Kopiert!';
                const originalLabel = copyBtn.getAttribute('data-copy-text') || 'Mermaid kopieren';
                if (copyText) copyText.textContent = copiedLabel;
                copyBtn.classList.add('btn-success', 'text-success-content');

                if (copyTimeout) clearTimeout(copyTimeout);
                copyTimeout = setTimeout(() => {
                    if (copyIcon) copyIcon.classList.remove('hidden');
                    if (copiedIcon) copiedIcon.classList.add('hidden');
                    if (copyText) copyText.textContent = originalLabel;
                    copyBtn.classList.remove('btn-success', 'text-success-content');
                }, 2000);
            } catch (err) {
                console.error('[Dataspace Graph] Copy failed:', err);
            }
        });
    }

    updateCopyButtonState();

    // Theme changes re-render
    const observer = new MutationObserver(() => {
        initMermaidConfig();
        fetchAndRenderGraph(true, true);
    });
    observer.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['data-theme', 'data-theme-mode']
    });

    // Watch table body mutations for active tags
    const loanBody = document.getElementById('loan-table-body');
    if (loanBody) {
        const tableObserver = new MutationObserver(() => {
            fetchAndRenderGraph(false, false);
        });
        tableObserver.observe(loanBody, { childList: true, subtree: true });
    }

    // Initial render
    fetchAndRenderGraph(true, true);
}
