import 'package:flutter/material.dart';

import '../../core/app_strings.dart';
import '../../core/theme.dart';
import '../../domain/models.dart';

class ConnectionPill extends StatelessWidget {
  const ConnectionPill({super.key, required this.online});

  final bool online;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final color = online ? const Color(0xFF07825F) : const Color(0xFFB54708);
    return Semantics(
      label: online ? strings.t('online') : strings.t('offline'),
      child: Container(
        padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
        decoration: BoxDecoration(
          color: color.withValues(alpha: 0.1),
          borderRadius: BorderRadius.circular(999),
        ),
        child: Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 7,
              height: 7,
              decoration: BoxDecoration(color: color, shape: BoxShape.circle),
            ),
            const SizedBox(width: 6),
            Text(
              online ? strings.t('online') : strings.t('offline'),
              style: TextStyle(
                color: color,
                fontWeight: FontWeight.w700,
                fontSize: 12,
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class StatusPill extends StatelessWidget {
  const StatusPill({super.key, required this.status});

  final SyncStatus status;

  @override
  Widget build(BuildContext context) {
    final strings = AppStrings.of(context);
    final (label, color) = switch (status) {
      SyncStatus.draft => (strings.t('draft'), Colors.blueGrey),
      SyncStatus.pendingSync => (strings.t('pending'), const Color(0xFFB54708)),
      SyncStatus.syncing => (strings.t('syncing'), AppTheme.teal),
      SyncStatus.synced => (strings.t('synced'), const Color(0xFF07825F)),
      SyncStatus.rejected => (
        strings.t('rejected'),
        Theme.of(context).colorScheme.error,
      ),
      SyncStatus.requiresReview => (
        strings.t('requiresReview'),
        const Color(0xFFB54708),
      ),
    };
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 9, vertical: 5),
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.09),
        borderRadius: BorderRadius.circular(999),
      ),
      child: Text(
        label,
        style: TextStyle(
          color: color,
          fontWeight: FontWeight.w700,
          fontSize: 11,
        ),
      ),
    );
  }
}

class EmptyState extends StatelessWidget {
  const EmptyState({
    super.key,
    required this.icon,
    required this.title,
    required this.message,
  });

  final IconData icon;
  final String title;
  final String message;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Container(
              width: 64,
              height: 64,
              decoration: const BoxDecoration(
                color: AppTheme.mint,
                shape: BoxShape.circle,
              ),
              child: Icon(icon, color: AppTheme.teal, size: 30),
            ),
            const SizedBox(height: 18),
            Text(
              title,
              style: Theme.of(
                context,
              ).textTheme.titleLarge?.copyWith(fontWeight: FontWeight.w800),
            ),
            const SizedBox(height: 6),
            Text(
              message,
              textAlign: TextAlign.center,
              style: const TextStyle(color: Colors.blueGrey, height: 1.4),
            ),
          ],
        ),
      ),
    );
  }
}

class SectionTitle extends StatelessWidget {
  const SectionTitle(this.text, {super.key, this.trailing});

  final String text;
  final Widget? trailing;

  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Expanded(
          child: Text(
            text,
            style: Theme.of(
              context,
            ).textTheme.titleMedium?.copyWith(fontWeight: FontWeight.w800),
          ),
        ),
        if (trailing != null) trailing!,
      ],
    );
  }
}

String compactDate(DateTime value) {
  const months = [
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec',
  ];
  final hour = value.hour.toString().padLeft(2, '0');
  final minute = value.minute.toString().padLeft(2, '0');
  return '${value.day} ${months[value.month - 1]} · $hour:$minute';
}
