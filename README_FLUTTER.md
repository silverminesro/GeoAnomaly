# GeoAnomaly Flutter Mobile App

This is the Flutter mobile application for the GeoAnomaly location-based AR game. The app connects to the GeoAnomaly backend to provide players with an inventory management system.

## Features

### Inventory Management
- **Player Statistics**: View XP, level, progress to next level
- **Artifacts Collection**: Browse and manage collected artifacts
- **Gear Collection**: View and organize collected gear
- **Item Details**: Detailed view of each item with images from Cloudflare R2

### Key Highlights
- **Efficient Image Loading**: Only loads one image per item to minimize costs
- **Level Progression**: Visual progress tracking with XP requirements
- **Rarity System**: Visual indicators for item rarity and gear levels
- **Biome Classification**: Items categorized by discovery biome
- **Real-time Data**: Connects to GeoAnomaly backend API

## API Integration

The app connects to the GeoAnomaly backend running on `localhost:8080` and uses the following endpoints:

- `/api/v1/auth/login` - Player authentication
- `/api/v1/user/profile` - Player profile and statistics
- `/api/v1/user/inventory` - Inventory items (artifacts and gear)
- `/api/v1/user/levels` - Level definitions and XP requirements

## Image Management

Images are loaded from Cloudflare R2 using the pattern:
```
https://your-cloudflare-domain.com/images/{itemType}/{itemId}.png
```

The app implements smart caching to ensure only one image is loaded per item, avoiding unnecessary charges.

## Configuration

Update the API base URL in `lib/core/services/api_service.dart`:
```dart
static const String baseUrl = 'https://your-backend-domain.com/api/v1';
```

Update the Cloudflare R2 domain in the `getImageUrl` method:
```dart
String getImageUrl(String itemType, String itemId) {
  return 'https://your-cloudflare-domain.com/images/$itemType/$itemId.png';
}
```

## Dependencies

- `flutter`: Flutter SDK
- `http`: HTTP client for API calls
- `cached_network_image`: Efficient image loading and caching
- `provider`: State management
- `shared_preferences`: Local storage
- `intl`: Date/time formatting

## Getting Started

1. Ensure Flutter is installed
2. Run `flutter pub get` to install dependencies
3. Configure API endpoints in the service files
4. Run `flutter run` to start the app

## Project Structure

```
lib/
├── main.dart                          # App entry point
├── core/
│   └── services/
│       └── api_service.dart           # Backend API integration
└── features/
    └── inventory/
        ├── models/
        │   └── inventory_models.dart   # Data models
        ├── services/
        │   └── inventory_service.dart  # Business logic
        ├── screens/
        │   ├── inventory_screen.dart   # Main inventory screen
        │   └── item_detail_screen.dart # Item detail view
        └── widgets/
            ├── inventory_item_card.dart # Item list card
            ├── player_stats_card.dart   # Player stats display
            ├── rarity_badge.dart        # Rarity indicators
            └── biome_chip.dart          # Biome indicators
```