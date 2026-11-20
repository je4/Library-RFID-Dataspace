/**
 * RFID Dataspace - Mermaid Knowledge Graph Visualizer Module
 * Connects physical RFID/NFC tag scans to Alma records and OpenAlex scholarly graph.
 */

import { initDataspaceReport, fetchAndRenderReport } from './dataspace-report.js';

let currentActiveTab = 'graph';
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

function updateActionButtonsState() {
    const output = document.getElementById('dataspace-mermaid-output');
    const hasSvg = !!output?.querySelector('svg');
    const hasMermaid = !!lastMermaid && lastMermaid.trim().length > 0;

    const copyBtn = document.getElementById('graph-copy-btn');
    if (copyBtn) {
        copyBtn.disabled = !hasMermaid;
    }

    const svgBtn = document.getElementById('graph-svg-btn');
    if (svgBtn) {
        svgBtn.disabled = !hasSvg;
    }
}

function downloadGraphSvg() {
    const output = document.getElementById('dataspace-mermaid-output');
    if (!output) return;
    const svg = output.querySelector('svg');
    if (!svg) return;

    // Clone SVG to strip pan/zoom transforms and interactive cursors for clean standalone vector file
    const clonedSvg = svg.cloneNode(true);
    clonedSvg.removeAttribute('style');
    clonedSvg.classList.remove('select-none');

    // Ensure standard XML and xlink namespaces
    if (!clonedSvg.getAttribute('xmlns')) {
        clonedSvg.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
    }
    if (!clonedSvg.getAttribute('xmlns:xlink')) {
        clonedSvg.setAttribute('xmlns:xlink', 'http://www.w3.org/1999/xlink');
    }

    const svgData = new XMLSerializer().serializeToString(clonedSvg);
    const svgBlob = new Blob([
        '<?xml version="1.0" encoding="UTF-8"?>\n',
        svgData
    ], { type: 'image/svg+xml;charset=utf-8' });

    const blobUrl = URL.createObjectURL(svgBlob);
    const downloadLink = document.createElement('a');
    downloadLink.href = blobUrl;

    const timestamp = new Date().toISOString().replace(/[:.]/g, '-').slice(0, 19);
    downloadLink.download = `dataspace-knowledge-graph-${timestamp}.svg`;

    document.body.appendChild(downloadLink);
    downloadLink.click();
    document.body.removeChild(downloadLink);
    setTimeout(() => URL.revokeObjectURL(blobUrl), 200);
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
            updateActionButtonsState();
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

                // Ensure all links open securely in new tabs
                svgElement.querySelectorAll('a').forEach(a => {
                    a.setAttribute('target', '_blank');
                    a.setAttribute('rel', 'noopener noreferrer');
                });
            }

            lastBarcodesKey = barcodesKey;
            lastRenderedDirection = currentDirection;
            lastMermaid = data.mermaid;
            updateActionButtonsState();

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
            updateActionButtonsState();
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

    let hasMovedDrag = false;
    let mouseDownClientX = 0;
    let mouseDownClientY = 0;

    // Pan drag interactions
    viewport.addEventListener('mousedown', (e) => {
        if (e.button !== 0 || !output.querySelector('svg')) return;
        isDragging = true;
        hasMovedDrag = false;
        mouseDownClientX = e.clientX;
        mouseDownClientY = e.clientY;
        startX = e.clientX - panX;
        startY = e.clientY - panY;
        viewport.style.cursor = 'grabbing';
    });

    window.addEventListener('mousemove', (e) => {
        if (!isDragging) return;
        if (Math.abs(e.clientX - mouseDownClientX) > 4 || Math.abs(e.clientY - mouseDownClientY) > 4) {
            hasMovedDrag = true;
        }
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

    // Prevent accidental link navigation when dragging/panning
    viewport.addEventListener('click', (e) => {
        if (hasMovedDrag) {
            e.preventDefault();
            e.stopPropagation();
            hasMovedDrag = false;
        }
    }, true);

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

    // Save SVG Button
    const svgBtn = document.getElementById('graph-svg-btn');
    const svgIcon = document.getElementById('graph-svg-icon');
    const svgSavedIcon = document.getElementById('graph-svg-saved-icon');
    const svgText = document.getElementById('graph-svg-text');
    let svgTimeout = null;

    if (svgBtn) {
        svgBtn.addEventListener('click', (e) => {
            e.preventDefault();
            try {
                downloadGraphSvg();
                if (svgIcon) svgIcon.classList.add('hidden');
                if (svgSavedIcon) svgSavedIcon.classList.remove('hidden');
                const savedLabel = svgBtn.getAttribute('data-saved-text') || 'SVG gespeichert!';
                const originalLabel = svgBtn.getAttribute('data-save-text') || 'SVG speichern';
                if (svgText) svgText.textContent = savedLabel;
                svgBtn.classList.add('btn-success', 'text-success-content');

                if (svgTimeout) clearTimeout(svgTimeout);
                svgTimeout = setTimeout(() => {
                    if (svgIcon) svgIcon.classList.remove('hidden');
                    if (svgSavedIcon) svgSavedIcon.classList.add('hidden');
                    if (svgText) svgText.textContent = originalLabel;
                    svgBtn.classList.remove('btn-success', 'text-success-content');
                }, 2000);
            } catch (err) {
                console.error('[Dataspace Graph] SVG download failed:', err);
            }
        });
    }

    // Tab Switching Handlers
    const tabBtnGraph = document.getElementById('tab-btn-graph');
    const tabBtnReport = document.getElementById('tab-btn-report');
    const graphToolbar = document.getElementById('graph-toolbar-controls');
    const reportToolbar = document.getElementById('report-toolbar-controls');
    const graphPane = document.getElementById('dataspace-graph-pane');
    const reportPane = document.getElementById('dataspace-report-pane');
    const subtitleEl = document.getElementById('dataspace-view-subtitle');

    const switchTab = (tab) => {
        currentActiveTab = tab;
        if (tab === 'graph') {
            if (tabBtnGraph) {
                tabBtnGraph.classList.add('tab-active', 'active');
                tabBtnGraph.setAttribute('aria-selected', 'true');
            }
            if (tabBtnReport) {
                tabBtnReport.classList.remove('tab-active', 'active');
                tabBtnReport.setAttribute('aria-selected', 'false');
            }
            if (graphToolbar) graphToolbar.classList.remove('hidden');
            if (reportToolbar) reportToolbar.classList.add('hidden');
            if (graphPane) graphPane.classList.remove('hidden');
            if (reportPane) reportPane.classList.add('hidden');
            updateTransform(output);
        } else {
            if (tabBtnReport) {
                tabBtnReport.classList.add('tab-active', 'active');
                tabBtnReport.setAttribute('aria-selected', 'true');
            }
            if (tabBtnGraph) {
                tabBtnGraph.classList.remove('tab-active', 'active');
                tabBtnGraph.setAttribute('aria-selected', 'false');
            }
            if (reportToolbar) reportToolbar.classList.remove('hidden');
            if (graphToolbar) graphToolbar.classList.add('hidden');
            if (reportPane) reportPane.classList.remove('hidden');
            if (graphPane) graphPane.classList.add('hidden');
            fetchAndRenderReport(false);
        }
    };

    if (tabBtnGraph) {
        tabBtnGraph.addEventListener('click', (e) => {
            e.preventDefault();
            switchTab('graph');
        });
    }

    if (tabBtnReport) {
        tabBtnReport.addEventListener('click', (e) => {
            e.preventDefault();
            switchTab('report');
        });
    }

    // Initialize report module
    initDataspaceReport();

    updateActionButtonsState();

    // Theme changes re-render
    const observer = new MutationObserver(() => {
        initMermaidConfig();
        fetchAndRenderGraph(true, true);
        if (currentActiveTab === 'report') {
            fetchAndRenderReport(true);
        }
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
            fetchAndRenderReport(false);
        });
        tableObserver.observe(loanBody, { childList: true, subtree: true });
    }

    // Initial render
    fetchAndRenderGraph(true, true);
    fetchAndRenderReport(true);
}
