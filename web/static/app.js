// Domain Check - Progressive Enhancement JavaScript
// This file is loaded asynchronously and adds live search features.

(function() {
    'use strict';

    // Check if JS is enabled
    document.documentElement.classList.add('js');

    // Enable TLD checkboxes (disabled by default for no-JS fallback)
    document.querySelectorAll('.tld-options input[type="checkbox"]').forEach(cb => {
        cb.disabled = false;
    });

    // Debounce helper
    function debounce(fn, delay) {
        let timeout;
        return function(...args) {
            clearTimeout(timeout);
            timeout = setTimeout(() => fn.apply(this, args), delay);
        };
    }

    // Live search (debounced, 3+ chars with TLD)
    const input = document.getElementById('domain-input');
    if (input) {
        // TODO: Add live search functionality
        // Will be implemented in Phase 3
    }

})();
