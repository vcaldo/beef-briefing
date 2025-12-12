/* Beef Dashboard - Custom JavaScript
 * Animation helpers and interactive enhancements
 */

// Wait for DOM to be ready
document.addEventListener('DOMContentLoaded', function() {
    // Initialize animations
    initializeAnimations();

    // Initialize number counters
    initializeCounters();
});

/**
 * Initialize intersection observer for scroll animations
 */
function initializeAnimations() {
    // Observe elements for fade-in animations
    const observerOptions = {
        root: null,
        rootMargin: '0px',
        threshold: 0.1
    };

    const observer = new IntersectionObserver((entries) => {
        entries.forEach(entry => {
            if (entry.isIntersecting) {
                entry.target.classList.add('visible');
                observer.unobserve(entry.target);
            }
        });
    }, observerOptions);

    // Observe chart containers
    document.querySelectorAll('.chart-container').forEach(el => {
        observer.observe(el);
    });
}

/**
 * Animate number counters
 */
function initializeCounters() {
    const counters = document.querySelectorAll('.stat-value');

    counters.forEach(counter => {
        // Create a MutationObserver to watch for content changes
        const observer = new MutationObserver((mutations) => {
            mutations.forEach(mutation => {
                if (mutation.type === 'characterData' || mutation.type === 'childList') {
                    const newValue = counter.textContent;
                    if (newValue && newValue !== '--') {
                        animateCounter(counter, newValue);
                    }
                }
            });
        });

        observer.observe(counter, {
            characterData: true,
            childList: true,
            subtree: true
        });
    });
}

/**
 * Animate a counter from 0 to target value
 */
function animateCounter(element, targetText) {
    // Parse the number from text (handle commas)
    const target = parseInt(targetText.replace(/,/g, ''), 10);

    if (isNaN(target)) {
        return;
    }

    const duration = 800;
    const start = 0;
    const startTime = performance.now();

    function update(currentTime) {
        const elapsed = currentTime - startTime;
        const progress = Math.min(elapsed / duration, 1);

        // Easing function (ease-out cubic)
        const easeOut = 1 - Math.pow(1 - progress, 3);

        const current = Math.floor(start + (target - start) * easeOut);
        element.textContent = current.toLocaleString();

        if (progress < 1) {
            requestAnimationFrame(update);
        } else {
            element.textContent = target.toLocaleString();
        }
    }

    requestAnimationFrame(update);
}

/**
 * Add ripple effect to buttons
 */
function addRippleEffect(button) {
    button.addEventListener('click', function(e) {
        const rect = button.getBoundingClientRect();
        const ripple = document.createElement('span');
        ripple.className = 'ripple';
        ripple.style.left = `${e.clientX - rect.left}px`;
        ripple.style.top = `${e.clientY - rect.top}px`;
        button.appendChild(ripple);

        setTimeout(() => ripple.remove(), 600);
    });
}

// Add ripple effect to period tabs and buttons
document.querySelectorAll('.period-tab, .lb-btn').forEach(addRippleEffect);

/**
 * Smooth scroll to section
 */
function scrollToSection(sectionId) {
    const section = document.getElementById(sectionId);
    if (section) {
        section.scrollIntoView({
            behavior: 'smooth',
            block: 'start'
        });
    }
}

/**
 * Format large numbers with abbreviations
 */
function formatNumber(num) {
    if (num >= 1000000) {
        return (num / 1000000).toFixed(1) + 'M';
    }
    if (num >= 1000) {
        return (num / 1000).toFixed(1) + 'K';
    }
    return num.toString();
}

/**
 * Debounce function for performance
 */
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Export functions for use in Dash callbacks if needed
window.dashboardHelpers = {
    formatNumber,
    scrollToSection,
    debounce
};
