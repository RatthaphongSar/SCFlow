# 403 Forbidden Error Fix & Role-Based Access Control Guide

## Problem Summary

You were getting `403 Forbidden` errors for these routes:
- `/logs` - System logs
- `/knowledge` - Knowledge base  
- `/users` - User management
- `/sql` - SQL scripts

## Root Cause

The application had **incorrect role names** in the route permission checks. The code was checking for roles like `"Admin"` and `"User"` which don't exist in the system.

### Before (Broken):
```go
logs := api.Group("/logs", middleware.RoleCheck("Admin", "Master"))
kb := api.Group("/knowledge", middleware.RoleCheck("Admin", "User", "Master"))
```

### After (Fixed):
```go
logs := api.Group("/logs", middleware.RoleCheck(models.RoleMaster, models.RoleProjectAdmin))
kb := api.Group("/knowledge", middleware.RoleCheck(models.RoleMaster, models.RoleProjectAdmin, models.RoleMember, models.RoleViewer))
```

## User Roles in the System

Your application has **4 role levels**:

| Role | Permissions | Description |
|------|------------|-------------|
| **Master** | Full system access | System administrator with all permissions |
| **ProjectAdmin** | Project management, logs, knowledge | Project lead with elevated permissions |
| **Member** | View/manage tasks, access knowledge | Regular team member |
| **Viewer** | View-only access, read knowledge | Read-only access, cannot make changes |

## Route Access Control

### Public Routes (No Authentication Required)
```
GET  /login        - Login page
POST /login        - Login submission
GET  /logout       - Logout
POST /webhook/line - LINE webhook (rate limited)
```

### Protected Routes (Requires any valid authentication)

#### All Authenticated Users Can Access:
```
GET  /           - Dashboard
GET  /tasks      - Task list
POST /tasks      - Create task
GET  /tasks/:id  - Task details
GET  /calendar   - Calendar view
GET  /projects   - Project list
```

### Role-Restricted Routes

#### Master & ProjectAdmin Can Access:
```
GET  /logs                 - System logs
POST /logs/analyze         - Log analysis
```

#### Master Only Can Access:
```
GET  /users                - User list
POST /users                - Create user
DELETE /users/:id          - Delete user

GET  /sql                  - SQL scripts
POST /sql                  - Create SQL script
GET  /sql/:id/content      - Get script content
POST /sql/run/:id          - Run SQL script
```

#### All Users Can Access:
```
GET  /knowledge            - Knowledge base
POST /knowledge            - Create knowledge article
GET  /knowledge/:id/content - Get article content
DELETE /knowledge/:id       - Delete article (if owner)
```

## How to Check Your Role

1. **Check in Browser Console:**
```javascript
// You can see your role if logged in
// It will be in the JWT token decoded
console.log(localStorage.getItem('user_role')); // If stored
// Or check the request headers being sent
```

2. **Database Query:**
```sql
SELECT id, username, role FROM users WHERE username = 'your_username';
```

## Common Solutions

### If You Get "Access Denied" Messages:

#### Solution 1: Check Your Current Role
```sql
SELECT username, role FROM users WHERE username = 'your_username';
```

#### Solution 2: Upgrade User Role
```sql
-- Make user a ProjectAdmin
UPDATE users SET role = 'ProjectAdmin' WHERE username = 'your_username';

-- Or make them Master (be careful!)
UPDATE users SET role = 'Master' WHERE username = 'your_username';
```

#### Solution 3: Check Default Admin Account
The system auto-creates a Master account on first run:
```sql
SELECT * FROM users WHERE role = 'Master';
```

If no Master user exists, check `internal/services/auth_service.go`:
```go
func SeedAdmin() {
    // Look for the admin seeding logic
}
```

## Error Messages You'll Now See

When you don't have permission:
- **Old**: "Access Denied: Insufficient Permissions"
- **New**: "Access Denied - You do not have permission to access this resource (403)"

This appears as a modern red alert notification in the top-right corner.

## Files Modified

1. **`cmd/server/main.go`**
   - Fixed `/logs` route: Now checks for `Master` or `ProjectAdmin`
   - Fixed `/knowledge` route: Now allows all authenticated users

2. **`public/js/htmx-alerts.js`**
   - Enhanced 403 error message for better UX
   - Added automatic redirect to login on 401 (session expired)

## Testing Your Fix

### Test with Master Account:
1. Login with Master role user
2. Try to access `/logs` - should work ✅
3. Try to access `/sql` - should work ✅
4. Try to access `/users` - should work ✅

### Test with ProjectAdmin Account:
1. Login with ProjectAdmin role user
2. Access `/logs` - should work ✅
3. Access `/knowledge` - should work ✅
4. Try `/users` - should show 403 ❌ (correct, not Master)
5. Try `/sql` - should show 403 ❌ (correct, only Master)

### Test with Member Account:
1. Login with Member role user
2. Access `/knowledge` - should work ✅
3. Access `/tasks` - should work ✅
4. Try `/logs` - should show 403 ❌ (correct)

## Role Permission Matrix

| Feature | Master | ProjectAdmin | Member | Viewer |
|---------|--------|--------------|--------|--------|
| View Dashboard | ✅ | ✅ | ✅ | ✅ |
| Manage Users | ✅ | ❌ | ❌ | ❌ |
| Run SQL Scripts | ✅ | ❌ | ❌ | ❌ |
| View Logs | ✅ | ✅ | ❌ | ❌ |
| Manage Projects | ✅ | ✅ | ✅ | ✅ |
| Manage Tasks | ✅ | ✅ | ✅ | ✅ |
| View Knowledge | ✅ | ✅ | ✅ | ✅ |
| Create Knowledge | ✅ | ✅ | ✅ | ❌ |

## Advanced: Custom Role Checks

If you need more granular control, you can add role checks at the handler level:

```go
func GetLogs(c *fiber.Ctx) error {
    userRole, _ := c.Locals("role").(string)
    
    // Additional check inside handler
    if userRole != "Master" && userRole != "ProjectAdmin" {
        return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
            "error": "Only Master and ProjectAdmin can view logs",
        })
    }
    
    // Continue with handler logic
    return nil
}
```

## Prevention: Best Practices

1. **Always use model constants** instead of string literals:
   ```go
   // ✅ Good
   middleware.RoleCheck(models.RoleMaster, models.RoleProjectAdmin)
   
   // ❌ Bad
   middleware.RoleCheck("Master", "Admin") // "Admin" doesn't exist!
   ```

2. **Define role constants once** - Don't hardcode role strings throughout the app

3. **Document role requirements** - Add comments to restricted routes

4. **Test role changes** - When you change a user's role, verify all their access permissions

5. **Audit logs** - Consider logging who accessed what and when

## Summary

Your 403 errors are now **fixed** by:
1. ✅ Using correct role names from models.go
2. ✅ Assigning appropriate roles to routes
3. ✅ Better error messages in the UI

Make sure your user account has the correct role for the feature you're trying to access!
