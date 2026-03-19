/**
 * Modern HTMX Confirmation Handler
 * Integrates the modern alert system with HTMX's hx-confirm attribute
 */

// Override HTMX's confirm behavior
if (typeof htmx !== 'undefined') {
    // Store original confirm handler
    const originalHtmxConfirm = htmx.config.historyCacheSize > 0 ? htmx.config : null;
    
    // Override the confirm method
    htmx.config.confirmPrompt = function(prompt, issuer) {
        return new Promise((resolve) => {
            Alert.confirm(
                'Confirm Action',
                prompt,
                () => {
                    resolve(true);
                },
                () => {
                    resolve(false);
                },
                {
                    confirmText: 'Confirm',
                    cancelText: 'Cancel',
                    confirmType: 'danger'
                }
            );
        });
    };
}

/**
 * Helper function to show success alerts from HTMX responses
 * Usage: Add hx-on="htmx:afterSwap:showSuccessAlert('Your message')"
 */
window.showSuccessAlert = (message) => {
    return () => Alert.success(message);
};

/**
 * Helper function to show error alerts from HTMX responses
 */
window.showErrorAlert = (message) => {
    return () => Alert.error(message);
};

/**
 * Helper to show alerts on response status
 * Usage: hx-on="htmx:afterOnLoad:handleResponseAlert(this, 'Success!', 'Error!')"
 */
window.handleResponseAlert = (element, successMsg, errorMsg) => {
    return () => {
        const lastRequest = element.getAttribute('hx-last-xhr');
        const response = element.htmx?.config?.xhr;
        if (response && response.status >= 200 && response.status < 300) {
            Alert.success(successMsg || 'Operation successful!');
        } else {
            Alert.error(errorMsg || 'Operation failed!');
        }
    };
};

/**
 * Global error handler for HTMX responses
 */
document.addEventListener('htmx:responseError', function(evt) {
    const status = evt.detail.xhr.status;
    let message = 'An error occurred';
    
    if (status === 400) {
        message = 'Invalid request - Check your input';
    } else if (status === 401) {
        message = 'Session expired - Please login again';
        // Redirect to login after 2 seconds
        setTimeout(() => window.location.href = '/login', 2000);
    } else if (status === 403) {
        message = 'Access Denied - You do not have permission to access this resource';
    } else if (status === 404) {
        message = 'Resource not found';
    } else if (status === 500) {
        message = 'Server error - Please try again later';
    }
    
    Alert.error(`${message} (${status})`);
});

/**
 * Auto-show modals when they're loaded via HTMX
 */
document.addEventListener('htmx:afterSwap', function(evt) {
    // If the target is #modal-content, activate the modal
    if (evt.detail.target.id === 'modal-content') {
        const modalContent = evt.detail.target;
        
        // Find alert-modal and overlay
        const modal = modalContent.querySelector('.alert-modal');
        const overlay = modalContent.querySelector('.alert-modal-overlay');
        
        if (modal) {
            // Use setTimeout to ensure the DOM is ready
            setTimeout(() => {
                modal.classList.add('show');
                if (overlay) {
                    overlay.classList.add('show');
                }
            }, 10);
        }
    }
});

/**
 * Show alerts on form submission errors
 */
document.addEventListener('htmx:validation:validate', function(evt) {
    if (!evt.detail.validate) {
        Alert.warning('Please fill in all required fields');
    }
});
