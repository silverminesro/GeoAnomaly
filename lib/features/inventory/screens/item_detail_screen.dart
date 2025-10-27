import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:provider/provider.dart';
import '../models/inventory_models.dart';
import '../services/inventory_service.dart';
import '../widgets/rarity_badge.dart';
import '../widgets/biome_chip.dart';

class ItemDetailScreen extends StatelessWidget {
  final InventoryItem item;

  const ItemDetailScreen({
    super.key,
    required this.item,
  });

  @override
  Widget build(BuildContext context) {
    final inventoryService = context.read<InventoryService>();
    final imageUrl = inventoryService.getItemImageUrl(item);

    return Scaffold(
      appBar: AppBar(
        title: Text(item.itemName),
        actions: [
          RarityBadge(rarity: item.itemRarity),
          const SizedBox(width: 16),
        ],
      ),
      body: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _buildImageSection(imageUrl),
            _buildDetailsSection(context),
            _buildPropertiesSection(context),
            _buildLocationSection(context),
          ],
        ),
      ),
    );
  }

  Widget _buildImageSection(String imageUrl) {
    return Container(
      width: double.infinity,
      height: 300,
      decoration: BoxDecoration(
        gradient: LinearGradient(
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
          colors: [
            Colors.black.withOpacity(0.1),
            Colors.black.withOpacity(0.3),
          ],
        ),
      ),
      child: Hero(
        tag: 'item_${item.id}',
        child: CachedNetworkImage(
          imageUrl: imageUrl,
          fit: BoxFit.contain,
          placeholder: (context, url) => const Center(
            child: CircularProgressIndicator(),
          ),
          errorWidget: (context, url, error) => Container(
            color: Colors.grey[800],
            child: Column(
              mainAxisAlignment: MainAxisAlignment.center,
              children: [
                Icon(
                  item.itemType == 'artifact' ? Icons.diamond : Icons.shield,
                  size: 64,
                  color: Colors.grey[600],
                ),
                const SizedBox(height: 8),
                Text(
                  'Image not available',
                  style: TextStyle(color: Colors.grey[600]),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildDetailsSection(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  item.itemName,
                  style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ),
              if (item.quantity > 1)
                Chip(
                  label: Text('x${item.quantity}'),
                  backgroundColor: Theme.of(context).colorScheme.secondary,
                ),
            ],
          ),
          const SizedBox(height: 8),
          Row(
            children: [
              Icon(
                item.itemType == 'artifact' ? Icons.diamond : Icons.shield,
                size: 20,
                color: Theme.of(context).colorScheme.primary,
              ),
              const SizedBox(width: 8),
              Text(
                _formatItemType(item.itemType),
                style: Theme.of(context).textTheme.titleMedium,
              ),
              const Spacer(),
              BiomeChip(biome: item.itemBiome),
            ],
          ),
          const SizedBox(height: 16),
          if (item.artifact != null) _buildArtifactDetails(context),
          if (item.gear != null) _buildGearDetails(context),
        ],
      ),
    );
  }

  Widget _buildArtifactDetails(BuildContext context) {
    final artifact = item.artifact!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildDetailRow(context, 'Type', _formatType(artifact.type)),
        _buildDetailRow(context, 'Rarity', _formatRarity(artifact.rarity)),
        _buildDetailRow(context, 'Biome', _formatBiome(artifact.biome)),
        if (artifact.exclusiveToBiome)
          _buildDetailRow(context, 'Exclusive', 'Biome Exclusive'),
      ],
    );
  }

  Widget _buildGearDetails(BuildContext context) {
    final gear = item.gear!;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        _buildDetailRow(context, 'Type', _formatType(gear.type)),
        _buildDetailRow(context, 'Level', 'Level ${gear.level}'),
        _buildDetailRow(context, 'Biome', _formatBiome(gear.biome)),
        if (gear.exclusiveToBiome)
          _buildDetailRow(context, 'Exclusive', 'Biome Exclusive'),
      ],
    );
  }

  Widget _buildDetailRow(BuildContext context, String label, String value) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 4),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          SizedBox(
            width: 80,
            child: Text(
              '$label:',
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          Expanded(
            child: Text(
              value,
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildPropertiesSection(BuildContext context) {
    if (item.properties.isEmpty) return const SizedBox();

    final properties = item.properties;
    final filteredProperties = Map<String, dynamic>.from(properties)
      ..removeWhere((key, value) => key == 'name' || key == 'type');

    if (filteredProperties.isEmpty) return const SizedBox();

    return Card(
      margin: const EdgeInsets.all(16),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              'Properties',
              style: Theme.of(context).textTheme.titleMedium?.copyWith(
                fontWeight: FontWeight.bold,
              ),
            ),
            const SizedBox(height: 12),
            ...filteredProperties.entries.map((entry) => Padding(
              padding: const EdgeInsets.symmetric(vertical: 4),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  SizedBox(
                    width: 120,
                    child: Text(
                      '${_formatPropertyKey(entry.key)}:',
                      style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                    ),
                  ),
                  Expanded(
                    child: Text(
                      _formatPropertyValue(entry.value),
                      style: Theme.of(context).textTheme.bodyMedium,
                    ),
                  ),
                ],
              ),
            )),
          ],
        ),
      ),
    );
  }

  Widget _buildLocationSection(BuildContext context) {
    final location = item.artifact != null
        ? '${item.artifact!.locationLatitude.toStringAsFixed(6)}, ${item.artifact!.locationLongitude.toStringAsFixed(6)}'
        : item.gear != null
            ? '${item.gear!.locationLatitude.toStringAsFixed(6)}, ${item.gear!.locationLongitude.toStringAsFixed(6)}'
            : null;

    if (location == null) return const SizedBox();

    return Card(
      margin: const EdgeInsets.all(16),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(
                  Icons.location_on,
                  color: Theme.of(context).colorScheme.primary,
                ),
                const SizedBox(width: 8),
                Text(
                  'Discovery Location',
                  style: Theme.of(context).textTheme.titleMedium?.copyWith(
                    fontWeight: FontWeight.bold,
                  ),
                ),
              ],
            ),
            const SizedBox(height: 12),
            Text(
              location,
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                fontFamily: 'monospace',
              ),
            ),
            const SizedBox(height: 8),
            Text(
              'Acquired on ${_formatDate(item.acquiredAt)}',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
      ),
    );
  }

  String _formatItemType(String type) {
    return type.replaceFirst(type[0], type[0].toUpperCase());
  }

  String _formatType(String type) {
    return type.split('_').map((word) => 
      word.replaceFirst(word[0], word[0].toUpperCase())
    ).join(' ');
  }

  String _formatRarity(String rarity) {
    return rarity.replaceFirst(rarity[0], rarity[0].toUpperCase());
  }

  String _formatBiome(String biome) {
    return biome.replaceFirst(biome[0], biome[0].toUpperCase());
  }

  String _formatPropertyKey(String key) {
    return key.split('_').map((word) => 
      word.replaceFirst(word[0], word[0].toUpperCase())
    ).join(' ');
  }

  String _formatPropertyValue(dynamic value) {
    if (value is DateTime) {
      return _formatDate(value);
    } else if (value is int && value > 1000000000) {
      // Likely a timestamp
      return _formatDate(DateTime.fromMillisecondsSinceEpoch(value * 1000));
    }
    return value.toString();
  }

  String _formatDate(DateTime date) {
    return '${date.day}/${date.month}/${date.year} ${date.hour.toString().padLeft(2, '0')}:${date.minute.toString().padLeft(2, '0')}';
  }
}