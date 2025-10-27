import 'package:flutter/material.dart';

enum RarityBadgeSize { small, medium, large }

class RarityBadge extends StatelessWidget {
  final String rarity;
  final RarityBadgeSize size;

  const RarityBadge({
    super.key,
    required this.rarity,
    this.size = RarityBadgeSize.medium,
  });

  @override
  Widget build(BuildContext context) {
    final config = _getRarityConfig(rarity);
    final sizeConfig = _getSizeConfig(size);

    return Container(
      padding: EdgeInsets.symmetric(
        horizontal: sizeConfig.horizontalPadding,
        vertical: sizeConfig.verticalPadding,
      ),
      decoration: BoxDecoration(
        color: config.color,
        borderRadius: BorderRadius.circular(sizeConfig.borderRadius),
        boxShadow: [
          BoxShadow(
            color: config.color.withOpacity(0.3),
            blurRadius: 4,
            offset: const Offset(0, 2),
          ),
        ],
      ),
      child: Text(
        config.displayName,
        style: TextStyle(
          color: config.textColor,
          fontSize: sizeConfig.fontSize,
          fontWeight: FontWeight.bold,
        ),
      ),
    );
  }

  _RarityConfig _getRarityConfig(String rarity) {
    switch (rarity.toLowerCase()) {
      case 'common':
        return _RarityConfig(
          displayName: 'Common',
          color: Colors.grey[600]!,
          textColor: Colors.white,
        );
      case 'rare':
        return _RarityConfig(
          displayName: 'Rare',
          color: Colors.blue,
          textColor: Colors.white,
        );
      case 'epic':
        return _RarityConfig(
          displayName: 'Epic',
          color: Colors.purple,
          textColor: Colors.white,
        );
      case 'legendary':
        return _RarityConfig(
          displayName: 'Legendary',
          color: Colors.orange,
          textColor: Colors.white,
        );
      default:
        // Handle gear levels (level_1, level_2, etc.)
        if (rarity.startsWith('level_')) {
          final level = int.tryParse(rarity.substring(6)) ?? 1;
          return _RarityConfig(
            displayName: 'Lv.$level',
            color: _getGearLevelColor(level),
            textColor: Colors.white,
          );
        }
        return _RarityConfig(
          displayName: rarity.toUpperCase(),
          color: Colors.grey,
          textColor: Colors.white,
        );
    }
  }

  Color _getGearLevelColor(int level) {
    if (level >= 8) return Colors.red;
    if (level >= 6) return Colors.purple;
    if (level >= 4) return Colors.blue;
    if (level >= 2) return Colors.green;
    return Colors.grey;
  }

  _SizeConfig _getSizeConfig(RarityBadgeSize size) {
    switch (size) {
      case RarityBadgeSize.small:
        return _SizeConfig(
          fontSize: 10,
          horizontalPadding: 6,
          verticalPadding: 2,
          borderRadius: 8,
        );
      case RarityBadgeSize.medium:
        return _SizeConfig(
          fontSize: 12,
          horizontalPadding: 8,
          verticalPadding: 4,
          borderRadius: 10,
        );
      case RarityBadgeSize.large:
        return _SizeConfig(
          fontSize: 14,
          horizontalPadding: 12,
          verticalPadding: 6,
          borderRadius: 12,
        );
    }
  }
}

class _RarityConfig {
  final String displayName;
  final Color color;
  final Color textColor;

  _RarityConfig({
    required this.displayName,
    required this.color,
    required this.textColor,
  });
}

class _SizeConfig {
  final double fontSize;
  final double horizontalPadding;
  final double verticalPadding;
  final double borderRadius;

  _SizeConfig({
    required this.fontSize,
    required this.horizontalPadding,
    required this.verticalPadding,
    required this.borderRadius,
  });
}