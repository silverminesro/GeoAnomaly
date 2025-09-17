import 'package:flutter/material.dart';

enum BiomeChipSize { small, medium, large }

class BiomeChip extends StatelessWidget {
  final String biome;
  final BiomeChipSize size;

  const BiomeChip({
    super.key,
    required this.biome,
    this.size = BiomeChipSize.medium,
  });

  @override
  Widget build(BuildContext context) {
    final config = _getBiomeConfig(biome);
    final sizeConfig = _getSizeConfig(size);

    return Container(
      padding: EdgeInsets.symmetric(
        horizontal: sizeConfig.horizontalPadding,
        vertical: sizeConfig.verticalPadding,
      ),
      decoration: BoxDecoration(
        color: config.color.withOpacity(0.2),
        border: Border.all(color: config.color, width: 1),
        borderRadius: BorderRadius.circular(sizeConfig.borderRadius),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(
            config.icon,
            size: sizeConfig.iconSize,
            color: config.color,
          ),
          SizedBox(width: sizeConfig.spacing),
          Text(
            config.displayName,
            style: TextStyle(
              color: config.color,
              fontSize: sizeConfig.fontSize,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }

  _BiomeConfig _getBiomeConfig(String biome) {
    switch (biome.toLowerCase()) {
      case 'forest':
        return _BiomeConfig(
          displayName: 'Forest',
          color: Colors.green,
          icon: Icons.park,
        );
      case 'urban':
        return _BiomeConfig(
          displayName: 'Urban',
          color: Colors.grey,
          icon: Icons.location_city,
        );
      case 'industrial':
        return _BiomeConfig(
          displayName: 'Industrial',
          color: Colors.orange,
          icon: Icons.factory,
        );
      case 'desert':
        return _BiomeConfig(
          displayName: 'Desert',
          color: Colors.amber,
          icon: Icons.wb_sunny,
        );
      case 'mountain':
        return _BiomeConfig(
          displayName: 'Mountain',
          color: Colors.brown,
          icon: Icons.terrain,
        );
      case 'coastal':
        return _BiomeConfig(
          displayName: 'Coastal',
          color: Colors.blue,
          icon: Icons.waves,
        );
      case 'swamp':
        return _BiomeConfig(
          displayName: 'Swamp',
          color: Colors.teal,
          icon: Icons.water,
        );
      case 'wasteland':
        return _BiomeConfig(
          displayName: 'Wasteland',
          color: Colors.red,
          icon: Icons.warning,
        );
      case 'rocky':
        return _BiomeConfig(
          displayName: 'Rocky',
          color: Colors.blueGrey,
          icon: Icons.landscape,
        );
      default:
        return _BiomeConfig(
          displayName: biome.replaceFirst(biome[0], biome[0].toUpperCase()),
          color: Colors.grey,
          icon: Icons.place,
        );
    }
  }

  _SizeConfig _getSizeConfig(BiomeChipSize size) {
    switch (size) {
      case BiomeChipSize.small:
        return _SizeConfig(
          fontSize: 10,
          horizontalPadding: 6,
          verticalPadding: 2,
          borderRadius: 8,
          iconSize: 12,
          spacing: 2,
        );
      case BiomeChipSize.medium:
        return _SizeConfig(
          fontSize: 12,
          horizontalPadding: 8,
          verticalPadding: 4,
          borderRadius: 10,
          iconSize: 14,
          spacing: 4,
        );
      case BiomeChipSize.large:
        return _SizeConfig(
          fontSize: 14,
          horizontalPadding: 12,
          verticalPadding: 6,
          borderRadius: 12,
          iconSize: 16,
          spacing: 6,
        );
    }
  }
}

class _BiomeConfig {
  final String displayName;
  final Color color;
  final IconData icon;

  _BiomeConfig({
    required this.displayName,
    required this.color,
    required this.icon,
  });
}

class _SizeConfig {
  final double fontSize;
  final double horizontalPadding;
  final double verticalPadding;
  final double borderRadius;
  final double iconSize;
  final double spacing;

  _SizeConfig({
    required this.fontSize,
    required this.horizontalPadding,
    required this.verticalPadding,
    required this.borderRadius,
    required this.iconSize,
    required this.spacing,
  });
}