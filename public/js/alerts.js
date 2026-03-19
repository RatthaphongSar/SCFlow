/**
 * Modern Alert & Notification System
 * Provides toast notifications, alerts, confirmations, and modals
 */

const AlertSystem = (() => {
    const notificationContainer = 'alert-container';
    
    // Initialize notification container if it doesn't exist
    const initContainer = () => {
        if (!document.getElementById(notificationContainer)) {
            const container = document.createElement('div');
            container.id = notificationContainer;
            document.body.appendChild(container);
        }
    };

    /**
     * Show a toast notification
     * @param {string} message - The message to display
     * @param {string} type - 'success', 'error', 'warning', 'info' (default: 'info')
     * @param {number} duration - Duration in ms (default: 3000)
     * @param {function} callback - Optional callback when toast closes
     */
    const toast = (message, type = 'info', duration = 3000, callback = null) => {
        initContainer();
        
        const toastEl = document.createElement('div');
        toastEl.className = `alert-toast alert-${type}`;
        
        const iconMap = {
            'success': '✓',
            'error': '✕',
            'warning': '⚠',
            'info': 'ℹ'
        };
        
        toastEl.innerHTML = `
            <div class="alert-toast-content">
                <span class="alert-toast-icon">${iconMap[type]}</span>
                <span class="alert-toast-message">${escapeHtml(message)}</span>
                <button class="alert-toast-close" aria-label="Close">&times;</button>
            </div>
        `;

        const container = document.getElementById(notificationContainer);
        container.appendChild(toastEl);

        // Trigger animation
        setTimeout(() => toastEl.classList.add('show'), 10);

        const closeToast = () => {
            toastEl.classList.remove('show');
            setTimeout(() => {
                toastEl.remove();
                if (callback) callback();
            }, 300);
        };

        toastEl.querySelector('.alert-toast-close').addEventListener('click', closeToast);

        if (duration > 0) {
            setTimeout(closeToast, duration + 300);
        }

        return toastEl;
    };

    /**
     * Show success notification
     */
    const success = (message, duration = 3000) => toast(message, 'success', duration);

    /**
     * Show error notification
     */
    const error = (message, duration = 3000) => toast(message, 'error', duration);

    /**
     * Show warning notification
     */
    const warning = (message, duration = 3000) => toast(message, 'warning', duration);

    /**
     * Show info notification
     */
    const info = (message, duration = 3000) => toast(message, 'info', duration);

    /**
     * Show a modal alert dialog
     * @param {string} title - Title of the alert
     * @param {string} message - Message content
     * @param {string} type - 'success', 'error', 'warning', 'info'
     * @param {string} buttonText - Text for the OK button (default: 'OK')
     * @param {function} callback - Callback when alert is closed
     */
    const alert = (title, message, type = 'info', buttonText = 'OK', callback = null) => {
        return showModal({
            type: 'alert',
            title: title,
            content: message,
            alertType: type,
            actions: [
                {
                    label: buttonText,
                    primary: true,
                    callback: callback
                }
            ]
        });
    };

    /**
     * Show a confirmation dialog
     * @param {string} title - Title of the confirmation
     * @param {string} message - Confirmation message
     * @param {function} onConfirm - Callback if user confirms
     * @param {function} onCancel - Callback if user cancels
     * @param {Object} options - Additional options (confirmText, cancelText)
     */
    const confirm = (title, message, onConfirm, onCancel = null, options = {}) => {
        const {
            confirmText = 'Confirm',
            cancelText = 'Cancel',
            confirmType = 'danger'
        } = options;

        return showModal({
            type: 'confirm',
            title: title,
            content: message,
            actions: [
                {
                    label: cancelText,
                    callback: onCancel,
                    variant: 'secondary'
                },
                {
                    label: confirmText,
                    primary: true,
                    callback: onConfirm,
                    variant: confirmType
                }
            ]
        });
    };

    /**
     * Show a custom modal dialog
     * @param {Object} config - Configuration object
     */
    const showModal = (config) => {
        const {
            type = 'custom',
            title = '',
            content = '',
            alertType = 'info',
            actions = [],
            width = '500px',
            closeOnBackdrop = true
        } = config;

        initContainer();

        // Create modal structure
        const modalOverlay = document.createElement('div');
        modalOverlay.className = 'alert-modal-overlay';

        const modal = document.createElement('div');
        modal.className = `alert-modal alert-modal-${alertType} alert-modal-${type}`;
        modal.style.width = width;

        // Build modal header
        let headerHTML = '';
        if (title) {
            const iconMap = {
                'success': '✓',
                'error': '✕',
                'warning': '⚠',
                'info': 'ℹ'
            };

            const icon = alertType && iconMap[alertType] ? iconMap[alertType] : '';
            headerHTML = `
                <div class="alert-modal-header">
                    ${icon ? `<span class="alert-modal-icon alert-${alertType}">${icon}</span>` : ''}
                    <h2 class="alert-modal-title">${escapeHtml(title)}</h2>
                    <button class="alert-modal-close" aria-label="Close">&times;</button>
                </div>
            `;
        }

        // Build modal body
        let bodyHTML = `
            <div class="alert-modal-body">
                ${content}
            </div>
        `;

        // Build modal footer
        let footerHTML = '';
        if (actions.length > 0) {
            footerHTML = '<div class="alert-modal-footer">';
            actions.forEach(action => {
                const variant = action.variant || 'primary';
                const btnClass = action.primary ? 'btn btn-primary' : `btn btn-${variant}`;
                footerHTML += `<button class="${btnClass}" data-action="${action.label}">${escapeHtml(action.label)}</button>`;
            });
            footerHTML += '</div>';
        }

        modal.innerHTML = headerHTML + bodyHTML + footerHTML;

        // Close button handler
        const closeBtn = modal.querySelector('.alert-modal-close');
        const closeModal = () => {
            modal.classList.remove('show');
            modalOverlay.classList.remove('show');
            setTimeout(() => {
                modal.remove();
                modalOverlay.remove();
            }, 300);
        };

        if (closeBtn) {
            closeBtn.addEventListener('click', closeModal);
        }

        // Backdrop click to close
        if (closeOnBackdrop) {
            modalOverlay.addEventListener('click', closeModal);
            modal.addEventListener('click', (e) => e.stopPropagation());
        }

        // Action buttons
        modal.querySelectorAll('[data-action]').forEach((btn, index) => {
            btn.addEventListener('click', () => {
                if (actions[index].callback) {
                    actions[index].callback();
                }
                closeModal();
            });
        });

        // Add to DOM and trigger animation
        const container = document.getElementById(notificationContainer);
        container.appendChild(modalOverlay);
        container.appendChild(modal);

        setTimeout(() => {
            modalOverlay.classList.add('show');
            modal.classList.add('show');
        }, 10);

        return { modal, modalOverlay, close: closeModal };
    };

    /**
     * Utility function to escape HTML
     */
    const escapeHtml = (text) => {
        const div = document.createElement('div');
        div.textContent = text;
        return div.innerHTML;
    };

    // Public API
    return {
        toast,
        success,
        error,
        warning,
        info,
        alert,
        confirm,
        showModal
    };
})();

// Make it globally accessible
window.Alert = AlertSystem;
