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

export async function fetchAndRenderGraph(immediate = false) {
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
        if (barcodes.length === 0) {
            output.innerHTML = '';
            if (emptyState) emptyState.classList.remove('hidden');
            if (loadingState) loadingState.classList.add('hidden');
            if (badge) {
                badge.classList.add('hidden');
                badge.textContent = '0 Nodes';
            }
            return;
        }

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

            resetZoomAndPan(output);

            if (badge) {
                badge.textContent = `${data.nodeCount} Nodes • ${data.edgeCount} Edges`;
                badge.classList.remove('hidden');
            }
        } catch (err) {
            console.warn('[Dataspace Graph] Render warning:', err);
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
            fetchAndRenderGraph(true);
        });

        dirLRBtn.addEventListener('click', () => {
            if (currentDirection === 'LR') return;
            currentDirection = 'LR';
            dirLRBtn.className = 'btn btn-xs join-item btn-primary';
            dirTDBtn.className = 'btn btn-xs join-item btn-ghost';
            fetchAndRenderGraph(true);
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

    // Fullscreen toggle
    const fullscreenBtn = document.getElementById('graph-fullscreen-btn');
    if (fullscreenBtn) {
        fullscreenBtn.addEventListener('click', () => {
            const isFull = graphSection.classList.toggle('fixed');
            if (isFull) {
                graphSection.classList.add('inset-0', 'z-50', 'rounded-none', 'p-6', 'bg-base-100', 'shadow-2xl');
                document.body.style.overflow = 'hidden';
            } else {
                graphSection.classList.remove('inset-0', 'z-50', 'rounded-none', 'p-6', 'bg-base-100', 'shadow-2xl');
                document.body.style.overflow = '';
            }
            setTimeout(() => resetZoomAndPan(output), 100);
        });
    }

    // Refresh button
    document.getElementById('graph-refresh-btn')?.addEventListener('click', () => {
        fetchAndRenderGraph(true);
    });

    // Theme changes re-render
    const observer = new MutationObserver(() => {
        initMermaidConfig();
        fetchAndRenderGraph(true);
    });
    observer.observe(document.documentElement, {
        attributes: true,
        attributeFilter: ['data-theme', 'data-theme-mode']
    });

    // Watch table body mutations for active tags
    const loanBody = document.getElementById('loan-table-body');
    if (loanBody) {
        const tableObserver = new MutationObserver(() => {
            fetchAndRenderGraph(false);
        });
        tableObserver.observe(loanBody, { childList: true, subtree: true });
    }

    // Initial render
    fetchAndRenderGraph(true);
}
