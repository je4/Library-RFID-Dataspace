/* RFID Dataspace Frontend Interactions (HTMX 4) - ES Module Entry Point */

import { initAudio } from './modules/audio.js';
import { initTableActions } from './modules/table-actions.js';
import { initIsilFlyout } from './modules/isil-flyout.js';
import { initAfiModal } from './modules/afi-modal.js';
import { initSSE } from './modules/sse.js';
import { initThemeSelector } from './modules/theme-selector.js';
import { initEncodeController } from './modules/encode-controller.js';
import { initShortcuts } from './modules/shortcuts.js';
import { initCatalog } from './modules/catalog.js';
import { initGateAlarm } from './modules/gate-alarm.js';
import { initDataspaceGraph } from './modules/dataspace-graph.js';

export function initDataspace() {
    document.getElementById('language-selector')?.addEventListener('change', event => {
        const language = event.target.value;
        if (!['de', 'en', 'fr', 'it'].includes(language)) return;
        document.cookie = `rfid_dataspace_language=${language}; Path=/; Max-Age=31536000; SameSite=Lax`;
        document.cookie = `library_desk_language=${language}; Path=/; Max-Age=31536000; SameSite=Lax`;
        window.location.reload();
    });
    initAudio();
    initTableActions();
    initIsilFlyout();
    initAfiModal();
    initSSE();
    initThemeSelector();
    initEncodeController();
    initShortcuts();
    initCatalog();
    initGateAlarm();
    initDataspaceGraph();
}

if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', initDataspace);
} else {
    initDataspace();
}
