// static/theme.js

(function () {
    console.log("Theme script initialising...");

    function getInitialTheme() {
        try {
            const savedTheme = localStorage.getItem('theme');
            if (savedTheme) return savedTheme;
        } catch (e) {
            console.warn("localStorage access denied:", e);
        }

        const systemPrefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        return systemPrefersDark ? 'dark' : 'light';
    }

    const initialTheme = getInitialTheme();
    document.documentElement.setAttribute('data-theme', initialTheme);
    // Also set on body just in case
    document.addEventListener('DOMContentLoaded', () => {
        document.body.setAttribute('data-theme', initialTheme);
    });

    console.log("Initial theme applied:", initialTheme);

    function toggleTheme() {
        const currentTheme = document.documentElement.getAttribute('data-theme');
        const newTheme = currentTheme === 'dark' ? 'light' : 'dark';

        document.documentElement.setAttribute('data-theme', newTheme);
        document.body.setAttribute('data-theme', newTheme);

        try {
            localStorage.setItem('theme', newTheme);
        } catch (e) {
            console.warn("Could not save theme to localStorage:", e);
        }

        updateToggleButton(newTheme);
        console.log("Theme toggled to:", newTheme);
    }

    function updateToggleButton(theme) {
        const toggleBtn = document.getElementById('theme-toggle');
        if (toggleBtn) {
            toggleBtn.textContent = theme === 'dark' ? '☀️ Light Mode' : '🌙 Dark Mode';
        }
    }

    function init() {
        console.log("DOM ready, setting up toggle...");
        const toggleBtn = document.getElementById('theme-toggle');
        if (toggleBtn) {
            updateToggleButton(document.documentElement.getAttribute('data-theme'));
            toggleBtn.onclick = toggleTheme; // Use direct property to avoid issues
            console.log("Event listener attached to:", toggleBtn);
        } else {
            console.error("CRITICAL: Dark Mode button (#theme-toggle) not found in DOM!");
        }
    }

    if (document.readyState === 'loading') {
        document.addEventListener('DOMContentLoaded', init);
    } else {
        init();
    }
})();
