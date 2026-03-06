// Theme management for KeyForge
(function() {
    const STORAGE_KEY = 'keyforge-theme';

    function getSystemTheme() {
        return window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
    }

    function getStoredTheme() {
        return localStorage.getItem(STORAGE_KEY);
    }

    function applyTheme(theme) {
        document.documentElement.setAttribute('data-theme', theme);
        updateToggleIcon(theme);
    }

    function updateToggleIcon(theme) {
        const sunIcon = document.getElementById('sun-icon');
        const moonIcon = document.getElementById('moon-icon');
        if (sunIcon && moonIcon) {
            sunIcon.style.display = theme === 'dark' ? 'block' : 'none';
            moonIcon.style.display = theme === 'light' ? 'block' : 'none';
        }
    }

    function toggleTheme() {
        const current = document.documentElement.getAttribute('data-theme') || getSystemTheme();
        const next = current === 'dark' ? 'light' : 'dark';
        localStorage.setItem(STORAGE_KEY, next);
        applyTheme(next);
    }

    // Initialize on load - run immediately to prevent flash
    const stored = getStoredTheme();
    const system = getSystemTheme();
    const theme = stored || system;
    applyTheme(theme);

    // Listen for system theme changes
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', function(e) {
        if (!getStoredTheme()) {
            applyTheme(e.matches ? 'dark' : 'light');
        }
    });

    // Expose toggle function globally
    window.toggleTheme = toggleTheme;
})();
