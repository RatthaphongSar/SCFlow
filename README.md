# SCFlow - Internal Admin Tool

Internal Task Tracking & Operations System.

## Features
- **Task Management**: Kanban-style tracking, file attachments.
- **SQL Script Manager**: Safe, read-only default execution for Admins.
- **LINE Integration**: Webhook notifications and chat commands.
- **Authentication**: JWT-based RBAC (Master, Admin, Member, Viewer).
- **Performance**: SSR, Gzip, Optimized DB queries.

## Environment Variables
Create a `.env` file or set these system variables:
```bash
PORT=3000
DB_DSN=scflow.db
LINE_CHANNEL_SECRET=your_channel_secret
LINE_CHANNEL_TOKEN=your_channel_token
LINE_GROUP_ID=your_group_id_for_notifications
JWT_SECRET=your_super_secret_key
```

## User Roles & Credentials
The system auto-creates a default Master user on first run:
- **Username**: `admin`
- **Password**: `admin123`

| Role | Permissions |
|------|-------------|
| Master | Full Access, Run SQL, Manage Users |
| ProjectAdmin | Manage Tasks, Assign Users |
| Member | Update Status, Add Logs |
| Viewer | Read Only |

## Deployment
1. **Install Dependencies**
   ```bash
   go mod tidy
   ```

2. **Build**
   ```bash
   go build -o scflow.exe ./cmd/server
   ```

3. **Run**
   ```bash
   ./scflow.exe
   ```

## LINE Bot Commands
- `/tasks` - List active tasks
- `/status {id}` - Check task status
- `/deploy done {id}` - Mark task as deployed (Done)

## Security Notes
- **SQL Safety**: `DROP`, `DELETE`, `TRUNCATE` are blocked. 5-second timeout enforced.
- **Auth**: Passwords hashed with bcrypt. JWT stored in HttpOnly cookies.
- **HTTPS**: Recommended for production (Secure cookies enabled).

## Tech Stack
- **Go 1.21** + **Fiber v2**
- **SQLite** (Embedded)
- **HTMX** (Frontend)
