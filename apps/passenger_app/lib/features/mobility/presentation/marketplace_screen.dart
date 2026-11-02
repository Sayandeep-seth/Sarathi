import 'package:flutter/material.dart';

import '../../../core/theme/app_theme.dart';
import '../data/mobility_catalog.dart';
import 'mobility_widgets.dart';

class MarketplaceScreen extends StatefulWidget {
  const MarketplaceScreen({required this.rideCount, super.key});

  final int rideCount;

  @override
  State<MarketplaceScreen> createState() => _MarketplaceScreenState();
}

class _MarketplaceScreenState extends State<MarketplaceScreen> {
  String _category = 'Tour';
  final Map<String, int> _cart = {};

  List<ShopProduct> get _products => MobilityCatalog.products
      .where((product) => product.category == _category)
      .toList();

  int get _itemCount => _cart.values.fold(0, (total, count) => total + count);

  int get _cartTotal {
    var total = 0;
    for (final product in MobilityCatalog.products) {
      total += product.priceRupees * (_cart[product.name] ?? 0);
    }
    return total;
  }

  @override
  Widget build(BuildContext context) {
    final remaining = (20 - widget.rideCount).clamp(0, 20);
    final progress = (widget.rideCount / 20).clamp(0.0, 1.0);
    return SingleChildScrollView(
      padding: const EdgeInsets.fromLTRB(20, 16, 20, 32),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const MobilityHeader(
            eyebrow: 'Ride more, get more',
            title: 'The SARATHI shop.',
            subtitle: 'Thoughtful picks for getaways and celebrations.',
          ),
          const SizedBox(height: 20),
          _RewardsCard(
            rideCount: widget.rideCount,
            remaining: remaining,
            progress: progress,
          ),
          const SizedBox(height: 21),
          Row(
            children: [
              _CategoryButton(
                title: 'Tour essentials',
                selected: _category == 'Tour',
                onTap: () => setState(() => _category = 'Tour'),
              ),
              const SizedBox(width: 8),
              _CategoryButton(
                title: 'Festival & ceremony',
                selected: _category == 'Celebration',
                onTap: () => setState(() => _category = 'Celebration'),
              ),
            ],
          ),
          const SizedBox(height: 15),
          for (final product in _products) ...[
            _ProductCard(
              product: product,
              quantity: _cart[product.name] ?? 0,
              onAdd: () {
                setState(() {
                  _cart[product.name] = (_cart[product.name] ?? 0) + 1;
                });
              },
              onRemove: () {
                setState(() {
                  final count = _cart[product.name] ?? 0;
                  if (count <= 1) {
                    _cart.remove(product.name);
                  } else {
                    _cart[product.name] = count - 1;
                  }
                });
              },
            ),
            const SizedBox(height: 12),
          ],
          if (_itemCount > 0) ...[
            const SizedBox(height: 6),
            AppSurface(
              child: Column(
                children: [
                  Row(
                    children: [
                      Container(
                        width: 42,
                        height: 42,
                        decoration: BoxDecoration(
                          color: const Color(0xFFEAF2EF),
                          borderRadius: BorderRadius.circular(13),
                        ),
                        child: const Icon(
                          Icons.shopping_bag_outlined,
                          color: AppTheme.emerald,
                        ),
                      ),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text(
                          '$_itemCount ${_itemCount == 1 ? 'item' : 'items'} · ₹$_cartTotal',
                          style: Theme.of(context)
                              .textTheme
                              .titleSmall
                              ?.copyWith(fontWeight: FontWeight.w800),
                        ),
                      ),
                    ],
                  ),
                  const SizedBox(height: 12),
                  SizedBox(
                    width: double.infinity,
                    child: FilledButton(
                      onPressed: () => showDemoConfirmation(
                        context,
                        title: 'Demo cart ready',
                        detail:
                            'Your $_itemCount items total ₹$_cartTotal. Checkout and payment are not connected in this preview.',
                      ),
                      child: const Text('Review cart'),
                    ),
                  ),
                ],
              ),
            ),
          ],
          const SizedBox(height: 14),
          const SampleNotice(
            message:
                'Products, prices and rewards are examples. Orders, delivery and reward fulfilment are not available in this demo.',
          ),
        ],
      ),
    );
  }
}

class _RewardsCard extends StatelessWidget {
  const _RewardsCard({
    required this.rideCount,
    required this.remaining,
    required this.progress,
  });

  final int rideCount;
  final int remaining;
  final double progress;

  @override
  Widget build(BuildContext context) {
    final canClaim = remaining == 0;
    return Container(
      padding: const EdgeInsets.all(17),
      decoration: BoxDecoration(
        borderRadius: BorderRadius.circular(22),
        gradient: const LinearGradient(
          begin: Alignment.topLeft,
          end: Alignment.bottomRight,
          colors: [Color(0xFF145D50), Color(0xFF073B38)],
        ),
        boxShadow: const [
          BoxShadow(
            color: Color(0x2011423A),
            blurRadius: 18,
            offset: Offset(0, 8),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.card_giftcard_rounded, color: Color(0xFFA8E3B7)),
              const SizedBox(width: 9),
              Expanded(
                child: Text(
                  'Your ride rewards',
                  style: Theme.of(context).textTheme.titleMedium?.copyWith(
                        color: Colors.white,
                        fontWeight: FontWeight.w800,
                      ),
                ),
              ),
              Text(
                '$rideCount rides',
                style: Theme.of(context).textTheme.labelMedium?.copyWith(
                      color: AppTheme.translucent(Colors.white, 0.85),
                      fontWeight: FontWeight.w800,
                    ),
              ),
            ],
          ),
          const SizedBox(height: 14),
          ClipRRect(
            borderRadius: BorderRadius.circular(10),
            child: LinearProgressIndicator(
              value: progress,
              minHeight: 8,
              backgroundColor: AppTheme.translucent(Colors.white, 0.2),
              color: const Color(0xFFA8E3B7),
            ),
          ),
          const SizedBox(height: 10),
          Row(
            children: [
              Expanded(
                child: Text(
                  canClaim
                      ? 'You reached the demo gift milestone!'
                      : '$remaining more rides to unlock a sample gift',
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: AppTheme.translucent(Colors.white, 0.9),
                      ),
                ),
              ),
              if (canClaim)
                TextButton(
                  style: TextButton.styleFrom(
                    foregroundColor: Colors.white,
                    padding: const EdgeInsets.symmetric(horizontal: 8),
                  ),
                  onPressed: () => showDemoConfirmation(
                    context,
                    title: 'Demo gift unlocked',
                    detail:
                        'You reached 20 rides in this demo. Reward eligibility and gift fulfilment are not connected.',
                  ),
                  child: const Text('View gift'),
                ),
            ],
          ),
        ],
      ),
    );
  }
}

class _CategoryButton extends StatelessWidget {
  const _CategoryButton({
    required this.title,
    required this.selected,
    required this.onTap,
  });

  final String title;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Expanded(
      child: OutlinedButton(
        onPressed: onTap,
        style: OutlinedButton.styleFrom(
          foregroundColor:
              selected ? Colors.white : Theme.of(context).colorScheme.onSurface,
          backgroundColor: selected ? AppTheme.emerald : Colors.white,
          side: BorderSide(
            color: selected
                ? AppTheme.emerald
                : Theme.of(context).colorScheme.outlineVariant,
          ),
          minimumSize: const Size.fromHeight(48),
          padding: const EdgeInsets.symmetric(horizontal: 7),
        ),
        child: Text(
          title,
          textAlign: TextAlign.center,
          style: const TextStyle(fontSize: 12, fontWeight: FontWeight.w700),
        ),
      ),
    );
  }
}

class _ProductCard extends StatelessWidget {
  const _ProductCard({
    required this.product,
    required this.quantity,
    required this.onAdd,
    required this.onRemove,
  });

  final ShopProduct product;
  final int quantity;
  final VoidCallback onAdd;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) {
    return AppSurface(
      padding: const EdgeInsets.all(12),
      child: Row(
        children: [
          Container(
            width: 74,
            height: 78,
            decoration: BoxDecoration(
              color: product.color,
              borderRadius: BorderRadius.circular(16),
            ),
            child: Icon(product.icon, size: 34, color: AppTheme.emerald),
          ),
          const SizedBox(width: 12),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Container(
                  padding: const EdgeInsets.symmetric(
                    horizontal: 7,
                    vertical: 4,
                  ),
                  decoration: BoxDecoration(
                    color: product.color,
                    borderRadius: BorderRadius.circular(12),
                  ),
                  child: Text(
                    product.category.toUpperCase(),
                    style: Theme.of(context).textTheme.labelSmall?.copyWith(
                          fontSize: 9,
                          fontWeight: FontWeight.w800,
                        ),
                  ),
                ),
                const SizedBox(height: 5),
                Text(
                  product.name,
                  style: Theme.of(context)
                      .textTheme
                      .titleSmall
                      ?.copyWith(fontWeight: FontWeight.w800),
                ),
                const SizedBox(height: 2),
                Text(
                  product.description,
                  maxLines: 2,
                  overflow: TextOverflow.ellipsis,
                  style: Theme.of(context).textTheme.bodySmall?.copyWith(
                        color: Theme.of(context).colorScheme.onSurfaceVariant,
                      ),
                ),
                const SizedBox(height: 7),
                PriceText(product.priceRupees),
              ],
            ),
          ),
          const SizedBox(width: 6),
          quantity == 0
              ? IconButton.filledTonal(
                  onPressed: onAdd,
                  tooltip: 'Add ${product.name} to cart',
                  icon: const Icon(Icons.add_rounded),
                )
              : _QuantityStepper(
                  quantity: quantity,
                  onAdd: onAdd,
                  onRemove: onRemove,
                ),
        ],
      ),
    );
  }
}

class _QuantityStepper extends StatelessWidget {
  const _QuantityStepper({
    required this.quantity,
    required this.onAdd,
    required this.onRemove,
  });
  final int quantity;
  final VoidCallback onAdd;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) => Column(
        children: [
          IconButton(
            onPressed: onAdd,
            visualDensity: VisualDensity.compact,
            icon: const Icon(Icons.add_circle_outline_rounded),
          ),
          Text('$quantity',
              style: const TextStyle(fontWeight: FontWeight.w800)),
          IconButton(
            onPressed: onRemove,
            visualDensity: VisualDensity.compact,
            icon: const Icon(Icons.remove_circle_outline_rounded),
          ),
        ],
      );
}
