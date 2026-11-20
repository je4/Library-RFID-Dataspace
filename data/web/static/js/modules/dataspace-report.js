/**
 * RFID Dataspace - Dossier & Report Module
 * Renders structured bibliographic, archive (InK), and scholarly (OpenAlex) reports.
 */

let lastReportBarcodesKey = null;
let currentReportData = null;
let isFetchingReport = false;

function escapeHtml(str) {
    if (!str) return '';
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

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
        currentReportData = null;
        if (loadingEl) loadingEl.classList.add('hidden');
        if (emptyEl) emptyEl.classList.remove('hidden');
        if (outputEl) {
            outputEl.innerHTML = '';
            outputEl.classList.add('hidden');
        }
        if (printBtn) printBtn.disabled = true;
        return;
    }

    if (!force && barcodesKey === lastReportBarcodesKey && currentReportData) {
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
        const endpoint = `${basePath}/dataspace/report?barcodes=${encodeURIComponent(barcodes.join(','))}`;

        const res = await fetch(endpoint, {
            headers: { 'Accept': 'application/json' }
        });

        if (!res.ok) {
            throw new Error(`HTTP ${res.status}`);
        }

        const data = await res.json();
        currentReportData = data;
        lastReportBarcodesKey = barcodesKey;

        renderReportHTML(data);

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

function renderReportHTML(report) {
    const outputEl = document.getElementById('dataspace-report-output');
    const emptyEl = document.getElementById('dataspace-report-empty');
    if (!outputEl) return;

    if (!report || !report.items || report.items.length === 0) {
        outputEl.innerHTML = '';
        outputEl.classList.add('hidden');
        if (emptyEl) emptyEl.classList.remove('hidden');
        return;
    }

    if (emptyEl) emptyEl.classList.add('hidden');
    outputEl.classList.remove('hidden');

    const dateStr = report.generatedAt ? new Date(report.generatedAt).toLocaleString() : new Date().toLocaleString();

    let html = `
    <!-- Report Print & Screen Header -->
    <div class="dataspace-report-header flex flex-col sm:flex-row sm:items-center justify-between gap-4 pb-4 border-b border-base-300">
        <div>
            <div class="flex items-center gap-2">
                <span class="badge badge-primary font-mono text-xs uppercase tracking-wider">Dataspace Dossier</span>
                <span class="text-xs text-base-content/60">${dateStr}</span>
            </div>
            <h1 class="text-xl sm:text-2xl font-bold tracking-tight text-base-content mt-1">Bibliografischer & Wissenschaftlicher Bericht</h1>
            <p class="text-xs sm:text-sm text-base-content/70">Umfassende Verknüpfung der RFID-Medien mit Alma, Mediathek InK und dem OpenAlex Knowledge Graph.</p>
        </div>
        <div class="flex items-center gap-2 print:hidden">
            <button type="button" class="btn btn-sm btn-outline gap-1.5" onclick="window.print()">
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <polyline points="6 9 6 2 18 2 18 9"></polyline>
                    <path d="M6 18H4a2 2 0 0 1-2-2v-5a2 2 0 0 1 2-2h16a2 2 0 0 1 2 2v5a2 2 0 0 1-2 2h-2"></path>
                    <rect x="6" y="14" width="12" height="8"></rect>
                </svg>
                <span>Drucken</span>
            </button>
        </div>
    </div>

    <!-- Executive Metrics Summary Card -->
    <div class="dataspace-report-metrics grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        <div class="metric-card bg-base-200/50 border border-base-200 p-3 rounded-xl flex flex-col">
            <span class="text-[11px] font-semibold text-base-content/60 uppercase">Medien</span>
            <span class="text-2xl font-black text-primary">${report.itemCount || 0}</span>
            <span class="text-[10px] text-base-content/50">Auf dem Lesefeld</span>
        </div>
        <div class="metric-card bg-base-200/50 border border-base-200 p-3 rounded-xl flex flex-col">
            <span class="text-[11px] font-semibold text-base-content/60 uppercase">Alma Treffer</span>
            <span class="text-2xl font-black text-blue-600">${report.summary?.catalogHits || 0}</span>
            <span class="text-[10px] text-base-content/50">Katalogdatensätze</span>
        </div>
        <div class="metric-card bg-base-200/50 border border-base-200 p-3 rounded-xl flex flex-col">
            <span class="text-[11px] font-semibold text-base-content/60 uppercase">InK Archiv</span>
            <span class="text-2xl font-black text-pink-600">${report.summary?.inkHits || 0}</span>
            <span class="text-[10px] text-base-content/50">Mediathek Digital</span>
        </div>
        <div class="metric-card bg-base-200/50 border border-base-200 p-3 rounded-xl flex flex-col">
            <span class="text-[11px] font-semibold text-base-content/60 uppercase">Publikationen</span>
            <span class="text-2xl font-black text-indigo-600">${report.summary?.openAlexHits || 0}</span>
            <span class="text-[10px] text-base-content/50">OpenAlex Works</span>
        </div>
        <div class="metric-card bg-base-200/50 border border-base-200 p-3 rounded-xl flex flex-col">
            <span class="text-[11px] font-semibold text-base-content/60 uppercase">Autoren</span>
            <span class="text-2xl font-black text-amber-600">${report.authorCount || 0}</span>
            <span class="text-[10px] text-base-content/50">Forscher & Urheber</span>
        </div>
        <div class="metric-card bg-base-200/50 border border-base-200 p-3 rounded-xl flex flex-col">
            <span class="text-[11px] font-semibold text-base-content/60 uppercase">Themenfelder</span>
            <span class="text-2xl font-black text-purple-600">${report.topicCount || 0}</span>
            <span class="text-[10px] text-base-content/50">Fachdisziplinen</span>
        </div>
    </div>
    `;

    // Overview of prominent authors and research domains if available
    const domains = report.summary?.primaryDomains || [];
    const fields = report.summary?.primaryFields || [];
    const prominentAuthors = report.summary?.prominentAuthors || [];

    if (domains.length > 0 || fields.length > 0 || prominentAuthors.length > 0) {
        html += `
        <div class="dataspace-summary-chips bg-base-200/30 border border-base-200 p-4 rounded-xl flex flex-wrap gap-4">
        `;
        if (domains.length > 0 || fields.length > 0) {
            html += `
            <div class="flex-1 min-w-[240px]">
                <h4 class="text-xs font-bold text-base-content/70 uppercase mb-2">Forschungsgebiete & Disziplinen</h4>
                <div class="flex flex-wrap gap-1.5">
                    ${domains.map(d => `<span class="badge badge-sm badge-info gap-1"><span class="w-1.5 h-1.5 rounded-full bg-info-content"></span>${escapeHtml(d)}</span>`).join('')}
                    ${fields.map(f => `<span class="badge badge-sm badge-outline gap-1">${escapeHtml(f)}</span>`).join('')}
                </div>
            </div>
            `;
        }
        if (prominentAuthors.length > 0) {
            html += `
            <div class="flex-1 min-w-[240px]">
                <h4 class="text-xs font-bold text-base-content/70 uppercase mb-2">Beteiligte Hauptautoren & Forscher</h4>
                <div class="flex flex-wrap gap-1.5">
                    ${prominentAuthors.map(a => `<span class="badge badge-sm badge-warning badge-outline gap-1"><span class="w-1.5 h-1.5 rounded-full bg-warning"></span>${escapeHtml(a)}</span>`).join('')}
                </div>
            </div>
            `;
        }
        html += `</div>`;
    }

    // Shared cross-connections (if any)
    if (report.sharedLinks && report.sharedLinks.length > 0) {
        html += `
        <div class="dataspace-shared-connections bg-primary/5 border border-primary/20 p-4 rounded-xl">
            <h3 class="text-sm font-bold text-primary flex items-center gap-1.5 mb-2">
                <svg class="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
                    <circle cx="18" cy="5" r="3"></circle>
                    <circle cx="6" cy="12" r="3"></circle>
                    <circle cx="18" cy="19" r="3"></circle>
                    <line x1="8.59" y1="13.51" x2="15.42" y2="17.49"></line>
                    <line x1="15.41" y1="6.51" x2="8.59" y2="10.49"></line>
                </svg>
                <span>Identifizierte Querverbindungen & Gemeinsamkeiten</span>
            </h3>
            <div class="grid grid-cols-1 md:grid-cols-2 gap-2">
                ${report.sharedLinks.map(link => `
                    <div class="bg-base-100 p-2.5 rounded-lg border border-base-200 text-xs flex items-start gap-2">
                        <span class="badge badge-xs badge-primary shrink-0 mt-0.5">${escapeHtml(link.relationType)}</span>
                        <div>
                            <span class="font-semibold">${escapeHtml(link.description)}</span>
                            <div class="text-[11px] text-base-content/60 mt-0.5">
                                <span>${escapeHtml(link.sourceTitle)}</span> ↔ <span>${escapeHtml(link.targetTitle)}</span>
                            </div>
                        </div>
                    </div>
                `).join('')}
            </div>
        </div>
        `;
    }

    // Media Items List
    html += `
    <div class="dataspace-report-items flex flex-col gap-6 mt-2">
        <h2 class="text-lg font-bold text-base-content border-b border-base-200 pb-2 flex items-center justify-between">
            <span>Detaillierte Dossiers der aufgelegten Medien</span>
            <span class="text-xs font-normal text-base-content/60">${report.items.length} Medium/Medien erfasst</span>
        </h2>
    `;

    report.items.forEach((item, idx) => {
        html += renderItemDossierCard(item, idx + 1);
    });

    html += `</div>`;

    outputEl.innerHTML = html;
}

function renderItemDossierCard(item, index) {
    const title = escapeHtml(item.title);
    const subtitle = escapeHtml(item.subtitle);
    const responsibility = escapeHtml(item.responsibility);
    const barcode = escapeHtml(item.barcode);
    const isbn = escapeHtml(item.isbn);
    const callNumber = escapeHtml(item.callNumber);
    const publisher = escapeHtml(item.publisher);
    const year = escapeHtml(item.publicationYear);
    const swisscoveryUrl = item.swisscoveryUrl || '#';

    let cardHtml = `
    <article class="dataspace-item-card bg-base-100 border border-base-300 rounded-2xl shadow-xs overflow-hidden">
        <!-- Item Header -->
        <div class="dataspace-item-card-header bg-base-200/40 px-5 py-3.5 border-b border-base-200 flex flex-wrap items-center justify-between gap-2">
            <div class="flex items-center gap-2">
                <span class="badge badge-primary font-mono text-xs">#${index}</span>
                <span class="badge badge-outline font-mono text-xs font-semibold">NFC: ${barcode}</span>
                ${item.hasCatalog ? '<span class="badge badge-info badge-sm gap-1">Alma Katalog</span>' : '<span class="badge badge-ghost badge-sm text-base-content/50">Kein Katalogtreffer</span>'}
            </div>
            ${item.hasCatalog ? `
            <a href="${swisscoveryUrl}" target="_blank" rel="noopener noreferrer" class="btn btn-xs btn-outline btn-primary gap-1 print:hidden" title="In Swisscovery öffnen">
                <span>Swisscovery</span>
                <svg class="w-3 h-3" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline><line x1="10" y1="14" x2="21" y2="3"></line></svg>
            </a>` : ''}
        </div>

        <!-- Bibliographic Body -->
        <div class="p-5 flex flex-col gap-4">
            <div class="dataspace-item-main flex flex-col gap-3">
                <div>
                    <h3 class="text-base sm:text-lg font-bold text-base-content leading-snug">${title}</h3>
                    ${subtitle ? `<p class="text-xs sm:text-sm text-base-content/80 mt-0.5 italic">${subtitle}</p>` : ''}
                    ${responsibility ? `<p class="text-xs text-base-content/70 mt-1 font-medium">${responsibility}</p>` : ''}
                </div>

                <!-- Metadata Table / Grid -->
                <div class="grid grid-cols-2 sm:grid-cols-4 gap-3 bg-base-200/30 p-3 rounded-xl text-xs">
                    <div>
                        <span class="text-base-content/50 block text-[10px] uppercase font-bold">Signatur</span>
                        <span class="font-mono font-semibold">${callNumber || '–'}</span>
                    </div>
                    <div>
                        <span class="text-base-content/50 block text-[10px] uppercase font-bold">ISBN</span>
                        <span class="font-mono font-semibold">${isbn || '–'}</span>
                    </div>
                    <div>
                        <span class="text-base-content/50 block text-[10px] uppercase font-bold">Verlag</span>
                        <span>${publisher || '–'}</span>
                    </div>
                    <div>
                        <span class="text-base-content/50 block text-[10px] uppercase font-bold">Jahr</span>
                        <span>${year || '–'}</span>
                    </div>
                </div>
            </div>
    `;

    // Authors & GND Authorities
    if (item.authors && item.authors.length > 0) {
        cardHtml += `
        <div class="dataspace-item-authors flex flex-col gap-1.5">
            <h4 class="text-xs font-bold text-base-content/70 uppercase">Autorinnen & Autoren (GND / OpenAlex)</h4>
            <div class="flex flex-wrap gap-2">
                ${item.authors.map(auth => `
                    <div class="inline-flex items-center gap-1.5 px-2.5 py-1 bg-amber-500/10 border border-amber-500/20 text-amber-900 dark:text-amber-200 rounded-lg text-xs font-medium">
                        <span>${escapeHtml(auth.name)}</span>
                        ${auth.gndId ? `
                            <a href="${auth.gndUrl}" target="_blank" rel="noopener noreferrer" class="badge badge-xs badge-neutral gap-0.5 text-[9px] hover:badge-primary" title="GND Authority Datensatz">
                                GND: ${escapeHtml(auth.gndId)}
                            </a>
                        ` : ''}
                    </div>
                `).join('')}
            </div>
        </div>
        `;
    }

    // Subjects / Topics
    if (item.subjects && item.subjects.length > 0) {
        cardHtml += `
        <div class="dataspace-item-subjects flex flex-col gap-1.5">
            <h4 class="text-xs font-bold text-base-content/70 uppercase">Schlagwörter & Themengebiete</h4>
            <div class="flex flex-wrap gap-1.5">
                ${item.subjects.map(s => `
                    <span class="badge badge-sm badge-secondary badge-outline gap-1">
                        ${escapeHtml(s.label)}
                    </span>
                `).join('')}
            </div>
        </div>
        `;
    }

    // InK Repository Matches
    if (item.inkItems && item.inkItems.length > 0) {
        cardHtml += `
        <div class="dataspace-ink-section mt-2 pt-3 border-t border-base-200">
            <h4 class="text-xs font-bold text-pink-700 dark:text-pink-400 uppercase flex items-center gap-1.5 mb-2">
                <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><rect x="3" y="3" width="18" height="18" rx="2" ry="2"></rect><line x1="3" y1="9" x2="21" y2="9"></line><line x1="9" y1="21" x2="9" y2="9"></line></svg>
                <span>Mediathek HGK InK & Archive (${item.inkItems.length} Treffer)</span>
            </h4>
            <div class="flex flex-col gap-2">
                ${item.inkItems.map(ink => `
                    <div class="dataspace-ink-entry p-3 bg-pink-500/5 border border-pink-500/20 rounded-xl text-xs flex flex-col gap-1.5">
                        <div class="flex flex-wrap items-center justify-between gap-1">
                            <span class="font-bold text-base-content">${escapeHtml(ink.title)}</span>
                            <a href="${ink.detailUrl}" target="_blank" rel="noopener noreferrer" class="badge badge-sm badge-primary gap-1 print:hidden">
                                InK Detail
                                <svg class="w-2.5 h-2.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline></svg>
                            </a>
                        </div>
                        <div class="flex flex-wrap gap-2 text-[11px] text-base-content/70">
                            <span><strong>Signatur:</strong> ${escapeHtml(ink.signature)}</span>
                            ${ink.type ? `<span><strong>Typ:</strong> ${escapeHtml(ink.type)}</span>` : ''}
                            ${ink.collectionTitle ? `<span><strong>Sammlung:</strong> ${escapeHtml(ink.collectionTitle)}</span>` : ''}
                            ${ink.date ? `<span><strong>Datum:</strong> ${escapeHtml(ink.date)}</span>` : ''}
                        </div>
                        ${ink.abstract ? `<p class="text-[11px] text-base-content/80 mt-1 line-clamp-2">${escapeHtml(ink.abstract)}</p>` : ''}
                    </div>
                `).join('')}
            </div>
        </div>
        `;
    }

    // OpenAlex Scholarly Knowledge Matches
    if (item.openAlexWorks && item.openAlexWorks.length > 0) {
        cardHtml += `
        <div class="dataspace-openalex-section mt-2 pt-3 border-t border-base-200">
            <h4 class="text-xs font-bold text-indigo-700 dark:text-indigo-400 uppercase flex items-center gap-1.5 mb-2">
                <svg class="w-3.5 h-3.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"></path><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"></path></svg>
                <span>Wissenschaftliche OpenAlex Publikationen (${item.openAlexWorks.length} Treffer)</span>
            </h4>
            <div class="flex flex-col gap-3">
                ${item.openAlexWorks.map(oa => `
                    <div class="dataspace-openalex-entry p-3 bg-indigo-500/5 border border-indigo-500/20 rounded-xl text-xs flex flex-col gap-2">
                        <div class="flex flex-wrap items-center justify-between gap-1">
                            <div>
                                <span class="font-bold text-base-content">${escapeHtml(oa.displayName)}</span>
                                ${oa.publicationYear ? `<span class="badge badge-xs badge-ghost ml-1">(${oa.publicationYear})</span>` : ''}
                            </div>
                            <div class="flex items-center gap-1.5 print:hidden">
                                ${oa.doiUrl ? `<a href="${oa.doiUrl}" target="_blank" rel="noopener noreferrer" class="badge badge-xs badge-outline">DOI</a>` : ''}
                                <a href="${oa.workUrl}" target="_blank" rel="noopener noreferrer" class="badge badge-xs badge-primary gap-0.5">
                                    OpenAlex
                                    <svg class="w-2.5 h-2.5" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M18 13v6a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V8a2 2 0 0 1 2-2h6"></path><polyline points="15 3 21 3 21 9"></polyline></svg>
                                </a>
                            </div>
                        </div>

                        ${oa.sourceName ? `
                        <div class="text-[11px] text-base-content/70">
                            <strong>Quelle / Verlag:</strong> ${escapeHtml(oa.sourceName)} ${oa.publisherName ? `(${escapeHtml(oa.publisherName)})` : ''}
                        </div>
                        ` : ''}

                        ${oa.primaryTopic ? `
                        <div class="text-[11px] flex flex-wrap items-center gap-1">
                            <span class="font-semibold text-purple-700 dark:text-purple-300">Primäres Thema:</span>
                            <span class="badge badge-xs badge-purple gap-1">${escapeHtml(oa.primaryTopic.label)}</span>
                            ${oa.primaryTopic.field ? `<span class="badge badge-xs badge-outline text-[10px]">${escapeHtml(oa.primaryTopic.field)}</span>` : ''}
                            ${oa.primaryTopic.domain ? `<span class="badge badge-xs badge-info badge-outline text-[10px]">${escapeHtml(oa.primaryTopic.domain)}</span>` : ''}
                        </div>
                        ` : ''}

                        ${oa.authorships && oa.authorships.length > 0 ? `
                        <div class="text-[11px] mt-0.5">
                            <strong class="text-base-content/70">Wissenschaftler & Affiliationen:</strong>
                            <div class="flex flex-wrap gap-1.5 mt-1">
                                ${oa.authorships.map(a => `
                                    <span class="inline-flex items-center gap-1 px-2 py-0.5 bg-base-200 rounded text-[11px]">
                                        <span>${escapeHtml(a.authorName)}</span>
                                        ${a.institutions && a.institutions.length > 0 ? `
                                            <span class="text-base-content/50 font-mono text-[10px]">(${escapeHtml(a.institutions[0].name)})</span>
                                        ` : ''}
                                    </span>
                                `).join('')}
                            </div>
                        </div>
                        ` : ''}
                    </div>
                `).join('')}
            </div>
        </div>
        `;
    }

    cardHtml += `
        </div>
    </article>
    `;

    return cardHtml;
}
