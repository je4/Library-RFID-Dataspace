/**
 * RFID Dataspace - Dossier & Report Module
 * Fetches and displays the server-rendered bibliographic and scholarly dossier.
 */

let lastReportBarcodesKey = null;
let isFetchingReport = false;

export function initDataspaceReport() {
    const printBtn = document.getElementById('report-print-btn');
    if (printBtn) {
        printBtn.addEventListener('click', () => {
            window.print();
        });
    }

    const refreshBtn = document.getElementById('report-refresh-btn');
    if (refreshBtn) {
        refreshBtn.addEventListener('click', () => {
            fetchAndRenderReport(true);
        });
    }
}

export async function fetchAndRenderReport(force = false) {
    const barcodes = getActiveBarcodes();
    const barcodesKey = barcodes.slice().sort().join(',');

    const loadingEl = document.getElementById('dataspace-report-loading');
    const emptyEl = document.getElementById('dataspace-report-empty');
    const outputEl = document.getElementById('dataspace-report-output');
    const printBtn = document.getElementById('report-print-btn');
    const badgeEl = document.getElementById('report-item-count-badge');

    if (badgeEl) {
        if (barcodes.length > 0) {
            badgeEl.textContent = `${barcodes.length} Items`;
            badgeEl.classList.remove('hidden');
        } else {
            badgeEl.classList.add('hidden');
        }
    }

    if (barcodes.length === 0) {
        lastReportBarcodesKey = '';
        if (loadingEl) loadingEl.classList.add('hidden');
        if (emptyEl) emptyEl.classList.remove('hidden');
        if (outputEl) {
            outputEl.innerHTML = '';
            outputEl.classList.add('hidden');
        }
        if (printBtn) printBtn.disabled = true;
        return;
    }

    if (!force && barcodesKey === lastReportBarcodesKey) {
        return; // Already up to date
    }

    if (isFetchingReport) return;
    isFetchingReport = true;

    if (loadingEl) loadingEl.classList.remove('hidden');
    if (emptyEl) emptyEl.classList.add('hidden');

    try {
        let basePath = window.location.pathname.replace(/\/+$/, '');
        if (basePath.endsWith('/dataspace') || basePath.endsWith('/loan')) {
            basePath = basePath.substring(0, basePath.lastIndexOf('/'));
        }
        const endpoint = `${basePath}/dataspace/report?format=html&barcodes=${encodeURIComponent(barcodes.join(','))}`;

        const res = await fetch(endpoint, {
            headers: { 'Accept': 'text/html' }
        });

        if (!res.ok) {
            throw new Error(`HTTP ${res.status}`);
        }

        const html = await res.text();
        lastReportBarcodesKey = barcodesKey;

        if (outputEl) {
            outputEl.innerHTML = html;
            outputEl.classList.remove('hidden');
        }
        if (emptyEl) emptyEl.classList.add('hidden');
        if (printBtn) printBtn.disabled = false;
    } catch (err) {
        console.error('Error fetching dataspace report:', err);
    } finally {
        isFetchingReport = false;
        if (loadingEl) loadingEl.classList.add('hidden');
    }
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
