# GeoAnomaly Flutter Configuration

This file contains configuration instructions for connecting the Flutter app to your GeoAnomaly backend.

## Backend Configuration

1. **Update API Base URL**
   
   Edit `lib/core/constants.dart`:
   ```dart
   static const String apiBaseUrl = 'https://your-backend-domain.com/api/v1';
   ```

2. **Cloudflare R2 Configuration**
   
   Edit `lib/core/constants.dart`:
   ```dart
   static const String cloudflareR2Domain = 'https://your-cloudflare-domain.com';
   ```

## Authentication Setup

1. **Login Credentials**
   
   Use your existing GeoAnomaly backend credentials:
   - Username: Your registered username
   - Password: Your account password

2. **Demo Credentials**
   
   Update demo credentials in `lib/core/constants.dart`:
   ```dart
   static const String demoUsername = 'your-username';
   static const String demoPasswordHint = '[Your password]';
   ```

## Image Storage Setup

The app expects images to be stored in Cloudflare R2 with the following structure:
```
https://your-cloudflare-domain.com/images/
├── artifact/
│   ├── {artifact-id-1}.png
│   ├── {artifact-id-2}.png
│   └── ...
└── gear/
    ├── {gear-id-1}.png
    ├── {gear-id-2}.png
    └── ...
```

## API Endpoints Used

The Flutter app connects to these GeoAnomaly backend endpoints:

- `POST /api/v1/auth/login` - Authentication
- `GET /api/v1/user/profile` - User profile and statistics
- `GET /api/v1/user/inventory` - Inventory items
- `GET /api/v1/user/levels` - Level definitions

## Testing the Connection

1. **Start your GeoAnomaly backend server**
2. **Update the API base URL** in constants.dart
3. **Run the Flutter app**
4. **Login with your credentials**
5. **Check that inventory data loads correctly**

## Troubleshooting

### Common Issues:

1. **Connection refused**
   - Verify backend server is running
   - Check API base URL is correct
   - Ensure no firewall blocking the connection

2. **Authentication failed**
   - Verify username and password are correct
   - Check backend user table for account

3. **Images not loading**
   - Verify Cloudflare R2 domain is correct
   - Check image files exist in expected structure
   - Verify R2 bucket is publicly accessible

4. **Empty inventory**
   - Check user has inventory items in database
   - Verify user_id matches between login and inventory
   - Check inventory_items table has data

### Backend API Requirements:

Ensure your backend implements these endpoints with the expected response formats shown in the Flutter models.

### Database Requirements:

The app expects these tables in your PostgreSQL database:
- `users` - User accounts and statistics
- `inventory_items` - Player's collected items
- `artifacts` - Artifact definitions
- `gear` - Gear definitions
- `level_definitions` - XP requirements and level info

Refer to `geoanomaly_schema.sql` for the complete database schema.