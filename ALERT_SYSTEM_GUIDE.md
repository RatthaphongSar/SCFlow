# Modern Alert & Notification System Guide

A comprehensive alert and notification system has been implemented to replace all basic `alert()` calls and improve modal dialogs throughout the application.

## Features

### 🎯 Toast Notifications
Non-intrusive notifications that appear in the top-right corner and auto-dismiss.

```javascript
// Success notification (auto-dismisses after 3 seconds)
Alert.success('Task saved successfully!');

// Error notification
Alert.error('Failed to update task');

// Warning notification
Alert.warning('This action cannot be undone');

// Info notification
Alert.info('New tasks assigned to you');

// Custom duration (in milliseconds)
Alert.success('Copied!', 2000); // Dismisses after 2 seconds

// With callback when dismissed
Alert.success('Done!', 3000, () => {
    console.log('Toast was dismissed');
});
```

### 📍 Alert Dialog
Modal dialog for important alerts with user confirmation.

```javascript
Alert.alert(
    'Success',                    // Title
    'Task created successfully',  // Message
    'success',                    // Type: 'success', 'error', 'warning', 'info'
    'OK',                        // Button text
    () => console.log('Closed')  // Callback
);

// Simplified version
Alert.alert('Error', 'Something went wrong', 'error');
```

### ✅ Confirmation Dialog
Ask user to confirm before performing action.

```javascript
Alert.confirm(
    'Delete Task?',
    'This action cannot be undone',
    () => {
        // User confirmed
        deleteTask();
    },
    () => {
        // User cancelled
        console.log('Cancelled');
    },
    {
        confirmText: 'Delete',
        cancelText: 'Keep',
        confirmType: 'danger'  // Button color: 'danger', 'warning', 'secondary'
    }
);
```

### 🎨 Custom Modal Dialog
Create completely custom modal dialogs.

```javascript
Alert.showModal({
    type: 'custom',
    title: 'Custom Dialog',
    content: '<p>Your custom HTML content here</p>',
    width: '600px',
    closeOnBackdrop: true,
    actions: [
        {
            label: 'Cancel',
            callback: () => console.log('Cancelled'),
            variant: 'secondary'
        },
        {
            label: 'Save',
            primary: true,
            callback: () => console.log('Saved'),
            variant: 'primary'
        }
    ]
});
```

## Usage in HTML

### 1. **With Regular JavaScript**

```html
<button onclick="Alert.success('Button clicked!')">Click Me</button>

<button onclick="Alert.confirm('Delete?', 'Are you sure?', () => deleteItem())">
    Delete
</button>
```

### 2. **With HTMX**

HTMX's automatic confirm dialogs are now integrated with the modern alert system:

```html
<!-- When clicked, will show modern confirmation dialog -->
<button hx-delete="/tasks/123" hx-confirm="Delete this task?">Delete</button>

<!-- Manual alerts with HTMX events -->
<form hx-post="/tasks" 
      hx-on="htmx:afterSwap: Alert.success('Task created successfully!')">
    <!-- form fields -->
</form>
```

### 3. **With Form Submissions**

```html
<form onsubmit="handleSubmit(event)">
    <!-- form fields -->
    <button type="submit">Submit</button>
</form>

<script>
function handleSubmit(event) {
    event.preventDefault();
    
    // Show processing
    Alert.info('Saving...');
    
    // Do submission
    fetch('/api/save', {
        method: 'POST',
        body: new FormData(event.target)
    })
    .then(response => {
        if (response.ok) {
            Alert.success('Saved successfully!');
            event.target.reset();
        } else {
            Alert.error('Save failed');
        }
    })
    .catch(err => Alert.error('Error: ' + err.message));
}
</script>
```

## Alert Types & Colors

| Type | Color | Icon | Usage |
|------|-------|------|-------|
| `success` | Green | ✓ | Successful operations |
| `error` | Red | ✕ | Errors and failures |
| `warning` | Orange | ⚠ | Warnings and cautions |
| `info` | Blue | ℹ | Information messages |

## Toast Notification Examples

```javascript
// Success - operation completed
Alert.success('Profile updated successfully!');

// Error - operation failed
Alert.error('Failed to send email');

// Warning - potential issue
Alert.warning('Changes will be lost if you leave now');

// Info - general information
Alert.info('New version available');

// Long message with custom duration
Alert.success(
    'Your report has been generated and will be emailed to you shortly.',
    5000  // Show for 5 seconds
);
```

## Modal Dialog Examples

### Success Alert
```javascript
Alert.alert(
    'Operation Complete',
    'Your changes have been saved successfully!',
    'success'
);
```

### Confirmation Before Delete
```javascript
Alert.confirm(
    'Delete Permanently?',
    'This action cannot be undone. All associated data will be lost.',
    () => {
        // Perform deletion
        console.log('Deleting...');
    },
    () => {
        // Cancelled
        console.log('Delete cancelled');
    },
    {
        confirmText: 'Yes, Delete',
        cancelText: 'No, Keep It',
        confirmType: 'danger'
    }
);
```

### Custom Form Modal
```javascript
Alert.showModal({
    title: 'Enter Your Name',
    content: '<input type="text" id="nameInput" placeholder="Your name..." style="width: 100%; padding: 10px; border: 1px solid #ccc; border-radius: 4px;">',
    actions: [
        {
            label: 'Cancel',
            variant: 'secondary'
        },
        {
            label: 'Submit',
            primary: true,
            callback: () => {
                const name = document.getElementById('nameInput').value;
                if (name) {
                    Alert.success(`Hello, ${name}!`);
                } else {
                    Alert.warning('Please enter your name');
                }
            }
        }
    ]
});
```

## Styling & Customization

The alert system uses CSS custom properties (variables) from your theme:

- `--primary-color`: Primary button and info alerts
- `--success-color`: Success alerts
- `--danger-color`: Error and danger alerts
- `--warning-color`: Warning alerts
- `--card-color`: Modal background
- `--border-color`: Modal borders
- `--text-color`: Text color
- `--text-muted`: Muted text

To customize colors, update these CSS variables in your `style.css`.

## API Reference

### `Alert.success(message, [duration], [callback])`
Show a success toast notification.

### `Alert.error(message, [duration], [callback])`
Show an error toast notification.

### `Alert.warning(message, [duration], [callback])`
Show a warning toast notification.

### `Alert.info(message, [duration], [callback])`
Show an info toast notification.

### `Alert.alert(title, message, [type], [buttonText], [callback])`
Show a modal alert dialog.

### `Alert.confirm(title, message, onConfirm, [onCancel], [options])`
Show a confirmation dialog.

**Options object:**
```javascript
{
    confirmText: 'Confirm',    // Text for confirm button
    cancelText: 'Cancel',      // Text for cancel button
    confirmType: 'danger'      // Button color variant
}
```

### `Alert.showModal(config)`
Display a custom modal dialog.

**Config object:**
```javascript
{
    type: 'custom',           // 'custom', 'alert', 'confirm'
    title: '',               // Modal title
    content: '',             // HTML content
    alertType: 'info',       // 'success', 'error', 'warning', 'info'
    width: '500px',          // Modal width
    closeOnBackdrop: true,   // Close when clicking overlay
    actions: [               // Array of action buttons
        {
            label: 'Button Text',
            callback: () => {},  // Function to call when clicked
            primary: false,      // Make it primary button
            variant: 'primary'   // Button variant
        }
    ]
}
```

## Responsive Behavior

The alerts automatically adapt for mobile devices:
- Toast notifications adjust from 400px max-width to full width
- Modal dialogs scale to 90% of screen width on mobile
- Buttons stack vertically on small screens
- Footer buttons become full-width on mobile

## Accessibility

The alert system includes:
- Proper ARIA labels
- Keyboard navigation support (Tab, Enter, Escape)
- Semantic HTML structure
- High contrast colors for readability
- Screen reader friendly messages

## Files

- `public/js/alerts.js` - Main alert system
- `public/js/htmx-alerts.js` - HTMX integration
- `public/css/style.css` - Alert styling (section: MODERN ALERT & NOTIFICATION SYSTEM)
- `views/layouts/main.html` - Script includes

## Migration Notes

Old code:
```javascript
alert('Operation complete!');
```

New code:
```javascript
Alert.success('Operation complete!');
```

Old code:
```html
<button onclick="if(confirm('Delete?')) deleteItem()">Delete</button>
```

New code:
```html
<button onclick="Alert.confirm('Delete?', 'Are you sure?', deleteItem)">Delete</button>
```

## Browser Support

- Chrome/Edge: Full support
- Firefox: Full support
- Safari: Full support
- Mobile browsers: Full support (iOS Safari, Chrome Mobile, Firefox Mobile)

## Performance

- Lightweight (~3KB minified for alerts.js)
- No external dependencies (except HTMX for integration)
- Efficient DOM manipulation
- GPU-accelerated animations
- Automatically removes DOM elements after animation

## Common Issues & Solutions

### Q: Modal not appearing?
A: Make sure `alerts.js` is loaded before using the Alert API.

### Q: Alerts appearing behind other elements?
A: The z-index is set to 10000 for modals and 9999 for overlays. Adjust if needed in CSS.

### Q: Need to access the modal element?
A: `Alert.showModal()` returns an object with `{modal, modalOverlay, close}` properties.

```javascript
const modal = Alert.showModal({ ... });
modal.close(); // Close the modal programmatically
```

## Support & Feedback

For issues or improvements, contact the development team.
