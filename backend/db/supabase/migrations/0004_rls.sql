-- Автор: Kuruma
-- =============================================================
-- 0004_rls.sql — Row-Level Security для всех таблиц витрины.
-- Принцип: с публичным anon-ключом разрешено только то, что
-- ДЕЙСТВИТЕЛЬНО нужно браузеру (читать активный каталог и активные
-- промокоды). Всё остальное — заказы, настройки, секреты, платежи —
-- доступно ТОЛЬКО service_role (наш backend ходит через Service Role
-- Key, который никогда не отдаём в браузер).
--
-- Применяется после 0003_integrations.sql. Идемпотентна:
-- drop policy if exists / create policy.
-- =============================================================

-- ---------------- store_catalog_items ----------------------------------------
alter table public.store_catalog_items enable row level security;

drop policy if exists "catalog_items_anon_read" on public.store_catalog_items;
create policy "catalog_items_anon_read" on public.store_catalog_items
  for select
  to anon, authenticated
  using (is_active = true);

drop policy if exists "catalog_items_service_full" on public.store_catalog_items;
create policy "catalog_items_service_full" on public.store_catalog_items
  for all
  to service_role
  using (true)
  with check (true);

-- ---------------- store_catalog_periods --------------------------------------
alter table public.store_catalog_periods enable row level security;

drop policy if exists "catalog_periods_anon_read" on public.store_catalog_periods;
create policy "catalog_periods_anon_read" on public.store_catalog_periods
  for select
  to anon, authenticated
  using (
    exists (
      select 1 from public.store_catalog_items i
      where i.id = item_id and i.is_active = true
    )
  );

drop policy if exists "catalog_periods_service_full" on public.store_catalog_periods;
create policy "catalog_periods_service_full" on public.store_catalog_periods
  for all
  to service_role
  using (true)
  with check (true);

-- ---------------- store_catalog_highlights -----------------------------------
alter table public.store_catalog_highlights enable row level security;

drop policy if exists "catalog_highlights_anon_read" on public.store_catalog_highlights;
create policy "catalog_highlights_anon_read" on public.store_catalog_highlights
  for select
  to anon, authenticated
  using (
    exists (
      select 1 from public.store_catalog_items i
      where i.id = item_id and i.is_active = true
    )
  );

drop policy if exists "catalog_highlights_service_full" on public.store_catalog_highlights;
create policy "catalog_highlights_service_full" on public.store_catalog_highlights
  for all
  to service_role
  using (true)
  with check (true);

-- ---------------- store_promo_codes ------------------------------------------
-- Внимание: anon НЕ должен видеть промокоды напрямую. Backend сам
-- проверяет промо при создании заказа. Поэтому никакого anon select.
alter table public.store_promo_codes enable row level security;

drop policy if exists "promo_codes_service_full" on public.store_promo_codes;
create policy "promo_codes_service_full" on public.store_promo_codes
  for all
  to service_role
  using (true)
  with check (true);

-- ---------------- store_orders -----------------------------------------------
-- Заказы: только service_role. Браузер общается с backend, backend
-- ходит в Supabase под service_role ключом.
alter table public.store_orders enable row level security;

drop policy if exists "orders_service_full" on public.store_orders;
create policy "orders_service_full" on public.store_orders
  for all
  to service_role
  using (true)
  with check (true);

-- ---------------- store_payment_transactions ---------------------------------
alter table public.store_payment_transactions enable row level security;

drop policy if exists "payment_tx_service_full" on public.store_payment_transactions;
create policy "payment_tx_service_full" on public.store_payment_transactions
  for all
  to service_role
  using (true)
  with check (true);

-- ---------------- store_integration_endpoints --------------------------------
alter table public.store_integration_endpoints enable row level security;

drop policy if exists "integration_endpoints_service_full" on public.store_integration_endpoints;
create policy "integration_endpoints_service_full" on public.store_integration_endpoints
  for all
  to service_role
  using (true)
  with check (true);

-- ---------------- store_integration_secrets ----------------------------------
alter table public.store_integration_secrets enable row level security;

drop policy if exists "integration_secrets_service_full" on public.store_integration_secrets;
create policy "integration_secrets_service_full" on public.store_integration_secrets
  for all
  to service_role
  using (true)
  with check (true);

-- ---------------- store_payment_settings -------------------------------------
alter table public.store_payment_settings enable row level security;

drop policy if exists "payment_settings_service_full" on public.store_payment_settings;
create policy "payment_settings_service_full" on public.store_payment_settings
  for all
  to service_role
  using (true)
  with check (true);

-- ---------------- Дополнительная защита --------------------------------------
-- Запрещаем выдачу прав на запись anon/authenticated на любые витринные
-- таблицы — даже если кто-то по ошибке выдаст GRANT, RLS заблокирует.
revoke insert, update, delete on
  public.store_catalog_items,
  public.store_catalog_periods,
  public.store_catalog_highlights,
  public.store_promo_codes,
  public.store_orders,
  public.store_payment_transactions,
  public.store_integration_endpoints,
  public.store_integration_secrets,
  public.store_payment_settings
from anon, authenticated;

-- service_role обходит RLS, поэтому ему явно ничего не выдаём.
