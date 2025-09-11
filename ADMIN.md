# Admin Panel

The admin panel provides a web interface for managing uploaded files on your Drop instance.

## Features

- **File Management**: View all uploaded files with metadata
- **File Details**: Detailed view of individual files with complete information
- **File Operations**: Update expiration dates, toggle one-time view, change original names
- **File Deletion**: Permanently delete files
- **Search & Filter**: Find files by name or other criteria
- **Sorting**: Sort files by various fields (name, size, upload date, expiration)

## Access

The admin panel is accessible at `/admin` on your Drop instance.

### Authentication

Admin access is configured through the `admin_password_hash` setting in your `config.yaml` file. The system uses Apache htpasswd format for password hashing.

⚠️ **IMPORTANT**: You must set up admin credentials before using the admin panel. There is no default password - you need to generate and configure the password hash.

### Setting Up Admin Password

To set up admin access, generate a password hash using htpasswd:

**Option 1: Command Line (if htpasswd is installed)**
```bash
# Generate password hash (replace 'admin' with your desired username)
htpasswd -n admin yourpassword

# Add the output to your config.yaml
admin_password_hash: "admin:$apr1$generatedhashhere"
```

**Option 2: Online Tool**
You can also generate the password hash online at [https://htpasswd.org/](https://htpasswd.org/) - just enter your username and password, select "Apache MD5" format, and copy the generated hash.

**Example:**
```bash
$ htpasswd -n admin mypassword
admin:$apr1$hOTejE2l$Au4wENmuj/hBpsjllVF9j1
```

Then add this to your `config.yaml`:
```yaml
admin_password_hash: "admin:$apr1$hOTejE2l$Au4wENmuj/hBpsjllVF9j1"
```

## Using the Admin Panel

### Dashboard
- Access the main dashboard at `/admin` after logging in
- View statistics cards showing total files, expired files, one-time files, and storage usage
- Browse all files in a sortable table with key information
- Use search to find specific files
- Adjust the number of files displayed per page

### File Management
- **View Details**: Click "View" on any file to see complete metadata
- **Update Settings**: Modify expiration dates, toggle one-time view, change original names
- **Delete Files**: Remove files permanently (with confirmation)
- **Direct Access**: Get direct links to files

### File Operations
- **Expiration Date**: Set custom expiration dates for files
- **One-Time View**: Toggle whether files can only be downloaded once
- **Original Name**: Change the display name of files
- **File Deletion**: Permanently remove files and their metadata

### Configuration Example

```yaml
# Enable admin panel
admin_panel_enabled: true

# Set admin credentials (htpasswd format)
admin_password_hash: "admin:$apr1$hOTejE2l$Au4wENmuj/hBpsjllVF9j1"
```
