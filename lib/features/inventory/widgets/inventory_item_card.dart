import 'package:flutter/material.dart';
import 'package:cached_network_image/cached_network_image.dart';
import 'package:provider/provider.dart';
import '../models/inventory_models.dart';
import '../services/inventory_service.dart';
import 'rarity_badge.dart';
import 'biome_chip.dart';

class InventoryItemCard extends StatelessWidget {
  final InventoryItem item;
  final VoidCallback? onTap;

  const InventoryItemCard({
    super.key,
    required this.item,
    this.onTap,
  });

  @override
  Widget build(BuildContext context) {
    final inventoryService = context.read<InventoryService>();
    final imageUrl = inventoryService.getItemImageUrl(item);

    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(12),
        child: Padding(
          padding: const EdgeInsets.all(16),
          child: Row(
            children: [
              _buildItemImage(imageUrl),
              const SizedBox(width: 16),
              Expanded(
                child: _buildItemInfo(context),
              ),
              _buildTrailingInfo(context),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildItemImage(String imageUrl) {
    return Container(
      width: 60,
      height: 60,
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(8),
        color: Colors.grey[800],
      ),
      child: ClipRRect(
        borderRadius: BorderRadius.circular(8),
        child: Hero(
          tag: 'item_${item.id}',
          child: CachedNetworkImage(
            imageUrl: imageUrl,
            fit: BoxFit.cover,
            placeholder: (context, url) => Container(
              color: Colors.grey[700],
              child: Icon(
                item.itemType == 'artifact' ? Icons.diamond : Icons.shield,
                color: Colors.grey[500],
              ),
            ),
            errorWidget: (context, url, error) => Container(
              color: Colors.grey[800],
              child: Icon(
                item.itemType == 'artifact' ? Icons.diamond : Icons.shield,
                color: Colors.grey[600],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildItemInfo(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          children: [
            Expanded(
              child: Text(
                item.itemName,
                style: Theme.of(context).textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.bold,
                ),
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
              ),
            ),
            RarityBadge(rarity: item.itemRarity, size: RarityBadgeSize.small),
          ],
        ),
        const SizedBox(height: 4),
        Row(
          children: [
            Icon(
              item.itemType == 'artifact' ? Icons.diamond : Icons.shield,
              size: 16,
              color: Theme.of(context).colorScheme.primary,
            ),
            const SizedBox(width: 4),
            Text(
              _formatItemType(item.itemType),
              style: Theme.of(context).textTheme.bodyMedium?.copyWith(
                color: Theme.of(context).colorScheme.onSurfaceVariant,
              ),
            ),
            const SizedBox(width: 12),
            BiomeChip(biome: item.itemBiome, size: BiomeChipSize.small),
          ],
        ),
        const SizedBox(height: 4),
        Text(
          'Acquired ${_formatDate(item.acquiredAt)}',
          style: Theme.of(context).textTheme.bodySmall?.copyWith(
            color: Theme.of(context).colorScheme.onSurfaceVariant,
          ),
        ),
      ],
    );
  }

  Widget _buildTrailingInfo(BuildContext context) {
    return Column(
      mainAxisAlignment: MainAxisAlignment.center,
      children: [
        if (item.quantity > 1)
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 4),
            decoration: BoxDecoration(
              color: Theme.of(context).colorScheme.secondary,
              borderRadius: BorderRadius.circular(12),
            ),
            child: Text(
              'x${item.quantity}',
              style: Theme.of(context).textTheme.bodySmall?.copyWith(
                color: Theme.of(context).colorScheme.onSecondary,
                fontWeight: FontWeight.bold,
              ),
            ),
          ),
        const SizedBox(height: 8),
        Icon(
          Icons.chevron_right,
          color: Theme.of(context).colorScheme.onSurfaceVariant,
        ),
      ],
    );
  }

  String _formatItemType(String type) {
    return type.replaceFirst(type[0], type[0].toUpperCase());
  }

  String _formatDate(DateTime date) {
    final now = DateTime.now();
    final difference = now.difference(date);

    if (difference.inDays > 7) {
      return '${date.day}/${date.month}/${date.year}';
    } else if (difference.inDays > 0) {
      return '${difference.inDays}d ago';
    } else if (difference.inHours > 0) {
      return '${difference.inHours}h ago';
    } else if (difference.inMinutes > 0) {
      return '${difference.inMinutes}m ago';
    } else {
      return 'Just now';
    }
  }
}