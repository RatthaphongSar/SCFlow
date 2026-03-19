# คำแนะนำการแก้ไข 403 Forbidden Errors / 403 Forbidden Error Fix Guide

## ไทย 🇹🇭

### ⚠️ ข้อผิดพลาด 403 หมายความว่า?

**403 Forbidden** = คุณล็อกอินสำเร็จ แต่ **ไม่มีสิทธิ์** เข้าถึงหน้านั้น

### 🔍 สาเหตุ

บัญชีของคุณมีบทบาท (Role) ที่ไม่มีสิทธิ์เพียงพอสำหรับหน้านั้น

### ✅ วิธีแก้ไข

#### วิธี 1: ใช้บัญชี Admin ดีฟอลต์
```
Username: admin
Password: admin123
```

#### วิธี 2: ตรวจสอบบทบาท (Role) ของคุณ
```sql
-- ดูบทบาทของคุณ
SELECT username, role FROM users WHERE username = 'YOUR_USERNAME';
```

#### วิธี 3: เปลี่ยนบทบาทเป็น Master
```sql
-- ให้สิทธิ์ Master ให้กับผู้ใช้
UPDATE users SET role = 'Master' WHERE username = 'YOUR_USERNAME';
```

### 🔑 บทบาท (Roles) ในระบบ

| บทบาท | สิทธิ์ |
|-------|-------|
| **Master** | เข้าถึงทั้งหมด (ผู้ดูแลระบบ) |
| **ProjectAdmin** | จัดการโปรเจค ดู Logs |
| **Member** | ใช้งานงาน ดู Knowledge |
| **Viewer** | อ่านอย่างเดียว |

### 📍 หน้าที่ต้องใช้ Role ไหน?

| หน้า | ต้องการ Role |
|-----|------------|
| `/logs` | Master หรือ ProjectAdmin |
| `/sql` | Master เท่านั้น |
| `/users` | Master เท่านั้น |
| `/knowledge` | ทุกคนที่ล็อกอิน |
| `/tasks` | ทุกคนที่ล็อกอิน |

### 🚀 ขั้นตอนแก้ไข

1. **ล็อกอินด้วย admin/admin123**
2. **ไปที่ `/users`**
3. **หาบัญชีของคุณ**
4. **เปลี่ยน Role เป็น Master**
5. **ออกจากระบบและล็อกอินใหม่**
6. **ลองใช้งานอีกครั้ง** ✅

---

## English 🇬🇧

### ⚠️ What Does 403 Forbidden Mean?

**403 Forbidden** = You're logged in successfully, but you don't have **permission** to access that page.

### 🔍 Root Cause

Your account has a role (permission level) that doesn't have enough access for that feature.

### ✅ How to Fix

#### Method 1: Use Default Admin Account
```
Username: admin
Password: admin123
```

#### Method 2: Check Your Role
```sql
-- Check your current role
SELECT username, role FROM users WHERE username = 'YOUR_USERNAME';
```

#### Method 3: Upgrade to Master Role
```sql
-- Give Master permissions to a user
UPDATE users SET role = 'Master' WHERE username = 'YOUR_USERNAME';
```

### 🔑 Roles in the System

| Role | Access Level |
|------|--------------|
| **Master** | Full system access (System Admin) |
| **ProjectAdmin** | Manage projects, view logs |
| **Member** | Work with tasks, view knowledge |
| **Viewer** | Read-only access |

### 📍 Which Role Needed for Each Page?

| Page | Required Role |
|------|---------------|
| `/logs` | Master or ProjectAdmin |
| `/sql` | Master only |
| `/users` | Master only |
| `/knowledge` | Any logged-in user |
| `/tasks` | Any logged-in user |

### 🚀 Quick Fix Steps

1. **Login with admin/admin123**
2. **Go to `/users`**
3. **Find your account**
4. **Change Role to Master**
5. **Logout and login again**
6. **Try again** ✅

### 🐛 Still Getting Errors?

**Check your browser console (F12) for:**
```
Response Status Error Code 403 from /logs
Response Status Error Code 403 from /sql
Response Status Error Code 403 from /users
```

**This means:**
- `/logs` - You need Master or ProjectAdmin role
- `/sql` - You need Master role only
- `/users` - You need Master role only

---

## Technical Details / รายละเอียดทางเทคนิค

### Role Permission Matrix

| Feature | Master | ProjectAdmin | Member | Viewer |
|---------|--------|--------------|--------|--------|
| Manage Users | ✅ | ❌ | ❌ | ❌ |
| Run SQL Scripts | ✅ | ❌ | ❌ | ❌ |
| View System Logs | ✅ | ✅ | ❌ | ❌ |
| Manage Projects | ✅ | ✅ | ✅ | ✅ |
| Manage Tasks | ✅ | ✅ | ✅ | ✅ |
| View Knowledge Base | ✅ | ✅ | ✅ | ✅ |
| Read-only Access | ✅ | ✅ | ✅ | ✅ |

### Default Admin Account

- **Username**: admin
- **Password**: admin123
- **Role**: Master
- **Purpose**: System administration and setup

⚠️ **Change this password in production!**

### Checking Active Sessions

If you're already logged in but still getting 403:
1. Press F12 to open Developer Tools
2. Go to "Storage" tab
3. Look for "jwt" cookie
4. The token should expire after 24 hours

If expired, logout and login again.
