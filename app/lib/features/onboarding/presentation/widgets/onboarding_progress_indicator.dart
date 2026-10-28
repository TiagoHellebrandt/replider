import 'package:flutter/material.dart';
import 'package:app/design_system/theme_extensions.dart';

class OnboardingProgressIndicator extends StatelessWidget {
  final int currentStep;
  final int totalSteps;

  const OnboardingProgressIndicator({
    super.key,
    required this.currentStep,
    this.totalSteps = 4,
  });

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).extension<AppColorsExtension>()!;

    return Row(
      mainAxisSize: MainAxisSize.min,
      children: List.generate(totalSteps, (index) {
        final bool isCurrent = index == currentStep;
        final bool isCompleted = index < currentStep;

        return Padding(
          padding: EdgeInsets.only(right: index < totalSteps - 1 ? 8 : 0),
          child: AnimatedContainer(
            duration: const Duration(milliseconds: 300),
            curve: Curves.easeInOut,
            width: isCurrent ? 24 : 12,
            height: 12,
            decoration: BoxDecoration(
              color: isCurrent
                  ? colors.bluePrimary
                  : isCompleted
                      ? colors.bluePrimary.withValues(alpha: 0.5)
                      : Colors.transparent,
              borderRadius: BorderRadius.circular(6),
              border: Border.all(
                color: isCurrent || isCompleted
                    ? colors.bluePrimary
                    : colors.borderMedium,
                width: 1.5,
              ),
            ),
          ),
        );
      }),
    );
  }
}
