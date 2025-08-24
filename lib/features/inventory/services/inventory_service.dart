import '../../../core/services/api_service.dart';
import '../models/inventory_models.dart';

class InventoryService {
  final ApiService _apiService;

  InventoryService(this._apiService);

  Future<User> getCurrentUser() async {
    final response = await _apiService.getUserProfile();
    return User.fromJson(response);
  }

  Future<List<InventoryItem>> getInventoryItems({
    int page = 1,
    int limit = 50,
    String? type,
  }) async {
    final response = await _apiService.getInventory(
      page: page,
      limit: limit,
      type: type,
    );

    final items = response['items'] as List<dynamic>? ?? [];
    return items.map((item) => InventoryItem.fromJson(item)).toList();
  }

  Future<List<InventoryItem>> getArtifacts({int page = 1, int limit = 50}) async {
    return getInventoryItems(page: page, limit: limit, type: 'artifact');
  }

  Future<List<InventoryItem>> getGear({int page = 1, int limit = 50}) async {
    return getInventoryItems(page: page, limit: limit, type: 'gear');
  }

  Future<List<LevelDefinition>> getLevelDefinitions() async {
    final response = await _apiService.getLevelDefinitions();
    final levels = response['levels'] as List<dynamic>? ?? [];
    return levels.map((level) => LevelDefinition.fromJson(level)).toList();
  }

  String getItemImageUrl(InventoryItem item) {
    return _apiService.getImageUrl(item.itemType, item.itemId);
  }

  Future<InventoryStats> getInventoryStats() async {
    final user = await getCurrentUser();
    final artifacts = await getArtifacts(limit: 1000);
    final gear = await getGear(limit: 1000);
    final levels = await getLevelDefinitions();

    final currentLevel = levels.firstWhere(
      (level) => level.level == user.level,
      orElse: () => LevelDefinition(
        level: 1,
        xpRequired: 0,
        featuresUnlocked: {},
        cosmeticUnlocks: {},
      ),
    );

    final nextLevel = levels.firstWhere(
      (level) => level.level == user.level + 1,
      orElse: () => LevelDefinition(
        level: user.level + 1,
        xpRequired: currentLevel.xpRequired + 1000,
        featuresUnlocked: {},
        cosmeticUnlocks: {},
      ),
    );

    return InventoryStats(
      user: user,
      totalArtifacts: artifacts.length,
      totalGear: gear.length,
      currentLevel: currentLevel,
      nextLevel: nextLevel,
      xpToNextLevel: nextLevel.xpRequired - user.xp,
      levelProgress: (user.xp - currentLevel.xpRequired) / 
                    (nextLevel.xpRequired - currentLevel.xpRequired),
    );
  }
}

class InventoryStats {
  final User user;
  final int totalArtifacts;
  final int totalGear;
  final LevelDefinition currentLevel;
  final LevelDefinition nextLevel;
  final int xpToNextLevel;
  final double levelProgress;

  InventoryStats({
    required this.user,
    required this.totalArtifacts,
    required this.totalGear,
    required this.currentLevel,
    required this.nextLevel,
    required this.xpToNextLevel,
    required this.levelProgress,
  });
}