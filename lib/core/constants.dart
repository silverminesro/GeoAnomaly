class AppConstants {
  // API Configuration
  static const String apiBaseUrl = 'http://localhost:8080/api/v1';
  static const String cloudflareR2Domain = 'https://your-cloudflare-domain.com';
  
  // Cloudflare R2 Configuration
  static String getImageUrl(String itemType, String itemId) {
    return '$cloudflareR2Domain/images/$itemType/$itemId.png';
  }
  
  // App Configuration
  static const String appName = 'GeoAnomaly';
  static const String appVersion = '1.0.0';
  
  // Authentication
  static const Duration tokenExpiryDuration = Duration(hours: 24);
  
  // Cache Configuration
  static const Duration imageCacheDuration = Duration(days: 7);
  static const int maxCacheSize = 100; // MB
  
  // UI Configuration
  static const double defaultPadding = 16.0;
  static const double cardBorderRadius = 12.0;
  static const double inputBorderRadius = 12.0;
  
  // Inventory Configuration
  static const int defaultPageSize = 50;
  static const int maxItemsPerPage = 100;
  
  // Demo Credentials
  static const String demoUsername = 'silverminesro';
  static const String demoPasswordHint = '[Your password]';
}