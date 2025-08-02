# Flutter Integration Guide for GeoAnomaly Media

## 🖼️ Loading Images in Flutter

### Public Endpoints (No Authentication Required)
For displaying artifact and gear images using `Image.network()`:

```dart
// Artifact images
Image.network(
  'https://your-api.com/api/v1/public/media/artifact/mushroom_sample',
  headers: {
    'Accept': 'image/jpeg,image/png,image/*',
  },
  loadingBuilder: (context, child, loadingProgress) {
    if (loadingProgress == null) return child;
    return CircularProgressIndicator();
  },
  errorBuilder: (context, error, stackTrace) {
    return Icon(Icons.error);
  },
)

// Generic images
Image.network(
  'https://your-api.com/api/v1/public/media/image/filename.jpg',
)
```

### Protected Endpoints (JWT Required)
For sensitive media that requires authentication:

```dart
// With dio package
final dio = Dio();
dio.options.headers['Authorization'] = 'Bearer $jwtToken';

final response = await dio.get(
  'https://your-api.com/api/v1/media/artifact/mushroom_sample',
  options: Options(responseType: ResponseType.bytes),
);

// Display the image
Image.memory(Uint8List.fromList(response.data))
```

### Caching Strategy
The server provides these cache headers:
- `Cache-Control: public, max-age=3600` (1 hour)
- `ETag` support for conditional requests

Use Flutter's `cached_network_image` package for better performance:

```dart
import 'package:cached_network_image/cached_network_image.dart';

CachedNetworkImage(
  imageUrl: 'https://your-api.com/api/v1/public/media/artifact/mushroom_sample',
  placeholder: (context, url) => CircularProgressIndicator(),
  errorWidget: (context, url, error) => Icon(Icons.error),
  cacheManager: DefaultCacheManager(),
  maxHeightDiskCache: 1000,
  maxWidthDiskCache: 1000,
)
```

## 🔧 Environment Configuration

For development, add these to your `.env`:
```bash
# Allow all origins in development
CORS_ALLOW_ALL=true

# R2 Storage Configuration
R2_ACCOUNT_ID=your_cloudflare_account_id
R2_ACCESS_KEY_ID=your_access_key
R2_SECRET_ACCESS_KEY=your_secret_key
R2_BUCKET_NAME=geoanomaly-artifacts
```

## 📱 Available Artifact Types

The following artifact types are available:

### Forest Artifacts
- `mushroom_sample`
- `tree_resin`
- `animal_bones`
- `herbal_extract`
- `dewdrop_pearl`

### Mountain Artifacts
- `mineral_ore`
- `crystal_shard`
- `stone_tablet`
- `mountain_herb`
- `ice_crystal`

### Industrial Artifacts
- `rusty_gear`
- `chemical_sample`
- `machinery_parts`
- `electronic_component`
- `toxic_waste`

### Urban Artifacts
- `old_documents`
- `medical_supplies`
- `electronics`
- `urban_artifact`
- `pocket_radio`

### Water Artifacts
- `water_sample`
- `aquatic_plant`
- `filtered_water`
- `abyss_pearl`
- `algae_biomass`

## 🚀 Performance Tips

1. **Use public endpoints** for images shown in lists or grids
2. **Implement image caching** to reduce bandwidth
3. **Resize images** on the server side if possible
4. **Use WebP format** for better compression (if supported)
5. **Preload critical images** during app initialization

## 🐛 Troubleshooting

### CORS Issues
If you get CORS errors:
1. Check that your Flutter web app's URL is in the allowed origins
2. For development, set `CORS_ALLOW_ALL=true` in server's `.env`
3. Make sure to handle preflight OPTIONS requests

### Image Not Loading
1. Check the artifact type exists in `internal/media/artifacts.go`
2. Verify the image file exists in R2 bucket under `artifacts/` prefix
3. Check server logs for detailed error messages
4. Ensure R2 credentials are properly configured

### Authentication Issues
1. Public endpoints don't require JWT tokens
2. For protected endpoints, ensure JWT token is valid and not expired
3. Include `Authorization: Bearer <token>` header for protected routes