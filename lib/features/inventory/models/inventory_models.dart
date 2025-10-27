class User {
  final String id;
  final String username;
  final String email;
  final int tier;
  final DateTime? tierExpires;
  final int xp;
  final int level;
  final int totalArtifacts;
  final int totalGear;
  final int zonesDiscovered;
  final bool isActive;
  final DateTime? lastLogin;
  final Map<String, dynamic> profileData;

  User({
    required this.id,
    required this.username,
    required this.email,
    required this.tier,
    this.tierExpires,
    required this.xp,
    required this.level,
    required this.totalArtifacts,
    required this.totalGear,
    required this.zonesDiscovered,
    required this.isActive,
    this.lastLogin,
    required this.profileData,
  });

  factory User.fromJson(Map<String, dynamic> json) {
    return User(
      id: json['id'],
      username: json['username'],
      email: json['email'],
      tier: json['tier'],
      tierExpires: json['tier_expires'] != null
          ? DateTime.parse(json['tier_expires'])
          : null,
      xp: json['xp'],
      level: json['level'],
      totalArtifacts: json['total_artifacts'],
      totalGear: json['total_gear'],
      zonesDiscovered: json['zones_discovered'],
      isActive: json['is_active'],
      lastLogin: json['last_login'] != null
          ? DateTime.parse(json['last_login'])
          : null,
      profileData: json['profile_data'] ?? {},
    );
  }
}

class LevelDefinition {
  final int level;
  final int xpRequired;
  final String? levelName;
  final Map<String, dynamic> featuresUnlocked;
  final Map<String, dynamic> cosmeticUnlocks;

  LevelDefinition({
    required this.level,
    required this.xpRequired,
    this.levelName,
    required this.featuresUnlocked,
    required this.cosmeticUnlocks,
  });

  factory LevelDefinition.fromJson(Map<String, dynamic> json) {
    return LevelDefinition(
      level: json['level'],
      xpRequired: json['xp_required'],
      levelName: json['level_name'],
      featuresUnlocked: json['features_unlocked'] ?? {},
      cosmeticUnlocks: json['cosmetic_unlocks'] ?? {},
    );
  }
}

class Artifact {
  final String id;
  final String zoneId;
  final String name;
  final String type;
  final String rarity;
  final double locationLatitude;
  final double locationLongitude;
  final Map<String, dynamic> properties;
  final bool isActive;
  final String biome;
  final bool exclusiveToBiome;

  Artifact({
    required this.id,
    required this.zoneId,
    required this.name,
    required this.type,
    required this.rarity,
    required this.locationLatitude,
    required this.locationLongitude,
    required this.properties,
    required this.isActive,
    required this.biome,
    required this.exclusiveToBiome,
  });

  factory Artifact.fromJson(Map<String, dynamic> json) {
    return Artifact(
      id: json['id'],
      zoneId: json['zone_id'],
      name: json['name'],
      type: json['type'],
      rarity: json['rarity'],
      locationLatitude: json['location_latitude'].toDouble(),
      locationLongitude: json['location_longitude'].toDouble(),
      properties: json['properties'] ?? {},
      isActive: json['is_active'],
      biome: json['biome'],
      exclusiveToBiome: json['exclusive_to_biome'],
    );
  }
}

class Gear {
  final String id;
  final String zoneId;
  final String name;
  final String type;
  final int level;
  final double locationLatitude;
  final double locationLongitude;
  final Map<String, dynamic> properties;
  final bool isActive;
  final String biome;
  final bool exclusiveToBiome;

  Gear({
    required this.id,
    required this.zoneId,
    required this.name,
    required this.type,
    required this.level,
    required this.locationLatitude,
    required this.locationLongitude,
    required this.properties,
    required this.isActive,
    required this.biome,
    required this.exclusiveToBiome,
  });

  factory Gear.fromJson(Map<String, dynamic> json) {
    return Gear(
      id: json['id'],
      zoneId: json['zone_id'],
      name: json['name'],
      type: json['type'],
      level: json['level'],
      locationLatitude: json['location_latitude'].toDouble(),
      locationLongitude: json['location_longitude'].toDouble(),
      properties: json['properties'] ?? {},
      isActive: json['is_active'],
      biome: json['biome'],
      exclusiveToBiome: json['exclusive_to_biome'],
    );
  }
}

class InventoryItem {
  final String id;
  final String userId;
  final String itemType; // 'artifact' or 'gear'
  final String itemId;
  final int quantity;
  final Map<String, dynamic> properties;
  final DateTime acquiredAt;
  final Artifact? artifact;
  final Gear? gear;

  InventoryItem({
    required this.id,
    required this.userId,
    required this.itemType,
    required this.itemId,
    required this.quantity,
    required this.properties,
    required this.acquiredAt,
    this.artifact,
    this.gear,
  });

  factory InventoryItem.fromJson(Map<String, dynamic> json) {
    return InventoryItem(
      id: json['id'],
      userId: json['user_id'],
      itemType: json['item_type'],
      itemId: json['item_id'],
      quantity: json['quantity'],
      properties: json['properties'] ?? {},
      acquiredAt: DateTime.parse(json['acquired_at']),
      artifact: json['artifact'] != null ? Artifact.fromJson(json['artifact']) : null,
      gear: json['gear'] != null ? Gear.fromJson(json['gear']) : null,
    );
  }

  String get itemName {
    if (artifact != null) return artifact!.name;
    if (gear != null) return gear!.name;
    return properties['name'] ?? 'Unknown Item';
  }

  String get itemRarity {
    if (artifact != null) return artifact!.rarity;
    if (gear != null) return 'level_${gear!.level}';
    return 'unknown';
  }

  String get itemBiome {
    if (artifact != null) return artifact!.biome;
    if (gear != null) return gear!.biome;
    return properties['zone_biome'] ?? 'unknown';
  }
}