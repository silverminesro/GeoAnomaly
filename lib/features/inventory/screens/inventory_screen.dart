import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../services/inventory_service.dart';
import '../models/inventory_models.dart';
import '../widgets/inventory_item_card.dart';
import '../widgets/player_stats_card.dart';
import 'item_detail_screen.dart';
import '../../core/services/auth_service.dart';

class InventoryScreen extends StatefulWidget {
  const InventoryScreen({super.key});

  @override
  State<InventoryScreen> createState() => _InventoryScreenState();
}

class _InventoryScreenState extends State<InventoryScreen>
    with SingleTickerProviderStateMixin {
  late TabController _tabController;
  bool _isLoading = true;
  String? _error;
  InventoryStats? _stats;
  List<InventoryItem> _artifacts = [];
  List<InventoryItem> _gear = [];

  @override
  void initState() {
    super.initState();
    _tabController = TabController(length: 3, vsync: this);
    _loadInventory();
  }

  Future<void> _loadInventory() async {
    setState(() {
      _isLoading = true;
      _error = null;
    });

    try {
      final inventoryService = context.read<InventoryService>();
      
      final [stats, artifacts, gear] = await Future.wait([
        inventoryService.getInventoryStats(),
        inventoryService.getArtifacts(),
        inventoryService.getGear(),
      ]);

      setState(() {
        _stats = stats as InventoryStats;
        _artifacts = artifacts as List<InventoryItem>;
        _gear = gear as List<InventoryItem>;
        _isLoading = false;
      });
    } catch (e) {
      setState(() {
        _error = e.toString();
        _isLoading = false;
      });
    }
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Inventory'),
        bottom: TabBar(
          controller: _tabController,
          tabs: const [
            Tab(icon: Icon(Icons.analytics), text: 'Stats'),
            Tab(icon: Icon(Icons.diamond), text: 'Artifacts'),
            Tab(icon: Icon(Icons.shield), text: 'Gear'),
          ],
        ),
        actions: [
          IconButton(
            onPressed: _loadInventory,
            icon: const Icon(Icons.refresh),
          ),
          PopupMenuButton<String>(
            onSelected: (value) {
              if (value == 'logout') {
                _logout();
              }
            },
            itemBuilder: (context) => [
              const PopupMenuItem(
                value: 'logout',
                child: Row(
                  children: [
                    Icon(Icons.logout),
                    SizedBox(width: 8),
                    Text('Logout'),
                  ],
                ),
              ),
            ],
          ),
        ],
      ),
      body: _isLoading
          ? const Center(child: CircularProgressIndicator())
          : _error != null
              ? Center(
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Icon(
                        Icons.error_outline,
                        size: 64,
                        color: Theme.of(context).colorScheme.error,
                      ),
                      const SizedBox(height: 16),
                      Text(
                        'Error loading inventory',
                        style: Theme.of(context).textTheme.headlineSmall,
                      ),
                      const SizedBox(height: 8),
                      Text(
                        _error!,
                        style: Theme.of(context).textTheme.bodyMedium,
                        textAlign: TextAlign.center,
                      ),
                      const SizedBox(height: 16),
                      ElevatedButton(
                        onPressed: _loadInventory,
                        child: const Text('Retry'),
                      ),
                    ],
                  ),
                )
              : TabBarView(
                  controller: _tabController,
                  children: [
                    _buildStatsTab(),
                    _buildArtifactsTab(),
                    _buildGearTab(),
                  ],
                ),
    );
  }

  Widget _buildStatsTab() {
    if (_stats == null) return const SizedBox();
    
    return SingleChildScrollView(
      padding: const EdgeInsets.all(16),
      child: Column(
        children: [
          PlayerStatsCard(stats: _stats!),
          const SizedBox(height: 16),
          _buildStatsGrid(),
        ],
      ),
    );
  }

  Widget _buildStatsGrid() {
    if (_stats == null) return const SizedBox();

    return GridView.count(
      shrinkWrap: true,
      physics: const NeverScrollableScrollPhysics(),
      crossAxisCount: 2,
      crossAxisSpacing: 16,
      mainAxisSpacing: 16,
      children: [
        _buildStatCard(
          'Total Artifacts',
          _stats!.totalArtifacts.toString(),
          Icons.diamond,
          Colors.purple,
        ),
        _buildStatCard(
          'Total Gear',
          _stats!.totalGear.toString(),
          Icons.shield,
          Colors.blue,
        ),
        _buildStatCard(
          'Zones Discovered',
          _stats!.user.zonesDiscovered.toString(),
          Icons.location_on,
          Colors.green,
        ),
        _buildStatCard(
          'Player Tier',
          'Tier ${_stats!.user.tier}',
          Icons.star,
          Colors.orange,
        ),
      ],
    );
  }

  Widget _buildStatCard(String title, String value, IconData icon, Color color) {
    return Card(
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(icon, size: 32, color: color),
            const SizedBox(height: 8),
            Text(
              value,
              style: Theme.of(context).textTheme.headlineMedium?.copyWith(
                fontWeight: FontWeight.bold,
                color: color,
              ),
            ),
            const SizedBox(height: 4),
            Text(
              title,
              style: Theme.of(context).textTheme.bodySmall,
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildArtifactsTab() {
    return _buildItemsList(_artifacts, 'artifacts');
  }

  Widget _buildGearTab() {
    return _buildItemsList(_gear, 'gear');
  }

  Widget _buildItemsList(List<InventoryItem> items, String type) {
    if (items.isEmpty) {
      return Center(
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(
              type == 'artifacts' ? Icons.diamond : Icons.shield,
              size: 64,
              color: Theme.of(context).disabledColor,
            ),
            const SizedBox(height: 16),
            Text(
              'No $type found',
              style: Theme.of(context).textTheme.headlineSmall,
            ),
            const SizedBox(height: 8),
            Text(
              'Start exploring to collect $type!',
              style: Theme.of(context).textTheme.bodyMedium,
            ),
          ],
        ),
      );
    }

    return RefreshIndicator(
      onRefresh: _loadInventory,
      child: ListView.builder(
        padding: const EdgeInsets.all(16),
        itemCount: items.length,
        itemBuilder: (context, index) {
          final item = items[index];
          return InventoryItemCard(
            item: item,
            onTap: () => _navigateToItemDetail(item),
          );
        },
      ),
    );
  }

  void _navigateToItemDetail(InventoryItem item) {
    Navigator.of(context).push(
      MaterialPageRoute(
        builder: (context) => ItemDetailScreen(item: item),
      ),
    );
  }

  Future<void> _logout() async {
    final authService = context.read<AuthService>();
    await authService.logout();
    
    if (mounted) {
      Navigator.of(context).pushReplacementNamed('/login');
    }
  }
}