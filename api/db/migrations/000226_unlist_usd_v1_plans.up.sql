-- The USD Solo/Core/Pro tiers are v1 self-serve attendance plans that were left
-- is_listed=true and Stripe-buyable, so /plans?currency=usd still publicly sold
-- them — with v1 caps (0 leads/mailboxes/funnels, drafts=100), i.e. the wrong
-- product. web/ is now pt-only contact-sales, so nothing legitimately links
-- them. Unlist so the only public tier is Enterprise (contact-sales); the SDR
-- contract tiers (Vendedor/Equipe/Escala) are assigned from the owner panel.
-- Rows are KEPT (FK/invoice history) — only hidden from the public catalogue.
UPDATE plans SET is_listed = false, updated_at = NOW()
WHERE currency = 'usd';
