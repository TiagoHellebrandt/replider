import 'package:flutter/material.dart';
import 'package:app/features/dashboard/domain/models/conversation_model.dart';

class SourceBadge extends StatelessWidget {
  final ConversationSource source;
  final double size;

  const SourceBadge({super.key, required this.source, this.size = 16});

  @override
  Widget build(BuildContext context) {
    final (icon, color) = switch (source) {
      ConversationSource.whatsapp => (Icons.chat, const Color(0xFF25D366)),
      // Blue, not green: in a mixed inbox the channel has to be readable at
      // a glance, and green already means WhatsApp everywhere else.
      ConversationSource.email => (
          Icons.alternate_email,
          const Color(0xFF3B82F6)
        ),
      ConversationSource.instagram => (
          Icons.camera_alt,
          const Color(0xFFE1306C)
        ),
      // Material's Icons.telegram is the actual paper-plane glyph from
      // Telegram's brand guidelines, not the generic Icons.send.
      ConversationSource.telegram => (Icons.telegram, const Color(0xFF0088CC)),
      // Twilio brand red — matches the SMS channel tile in settings.
      ConversationSource.sms => (Icons.sms, const Color(0xFFF22F46)),
      // Internal CS thread (canal-interno) — platform support channel.
      ConversationSource.internal => (
          Icons.support_agent,
          const Color(0xFF7C6CF2)
        ),
    };

    return Container(
      width: size + 6,
      height: size + 6,
      decoration: BoxDecoration(
        color: color.withValues(alpha: 0.15),
        borderRadius: BorderRadius.circular(4),
      ),
      child: Icon(icon, size: size, color: color),
    );
  }
}
