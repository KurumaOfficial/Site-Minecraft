-- =============================================================
-- schema.sql — кумулятивная схема витрины ESTELAR.SU
-- =============================================================
-- Сгенерирована из migrations/0001..0004 — содержит ИДЕНТИЧНЫЙ SQL,
-- просто склеенный для удобной разовой установки через Supabase
-- Dashboard → SQL Editor → New query → Paste → Run.
--
-- Если базу обновляете повторно — лучше открыть migrations/ и
-- применять файлы по одному (0001 → 0002 → 0003 → 0004), тогда видно,
-- что именно изменилось.
-- =============================================================


-- ============= migrations/0001_init.sql =============
-- =============================================================
-- 0001_init.sql — базовая схема витрины ESTELAR.SU
-- Применять ПЕРВОЙ. Идемпотентна (create if not exists / drop trigger if exists).
-- =============================================================
create extension if not exists pgcrypto;

create or replace function public.set_row_updated_at()
returns trigger
language plpgsql
as $$
begin
  new.updated_at = now();
  return new;
end;
$$;

-- ---------- Каталог -----------------------------------------------------------
create table if not exists public.store_catalog_items (
  id uuid primary key default gen_random_uuid(),
  external_id text not null unique,
  slug text not null unique,
  name text not null,
  category text not null check (category in ('privilege', 'case')),
  category_label text not null,
  summary text not null,
  image text not null,
  currency text not null default 'RUB',
  sort_order integer not null default 0,
  is_active boolean not null default true,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

create table if not exists public.store_catalog_periods (
  id uuid primary key default gen_random_uuid(),
  item_id uuid not null references public.store_catalog_items(id) on delete cascade,
  code text not null,
  label text not null,
  price integer not null check (price >= 0),
  is_default boolean not null default false,
  sort_order integer not null default 0,
  created_at timestamptz not null default now(),
  unique (item_id, code)
);

create table if not exists public.store_catalog_highlights (
  id bigint generated always as identity primary key,
  item_id uuid not null references public.store_catalog_items(id) on delete cascade,
  sort_order integer not null default 0,
  text text not null
);

-- ---------- Промокоды ---------------------------------------------------------
create table if not exists public.store_promo_codes (
  code text primary key,
  discount_percent integer not null check (discount_percent > 0 and discount_percent <= 100),
  is_active boolean not null default true,
  usage_limit integer,
  times_used integer not null default 0,
  starts_at timestamptz,
  ends_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- ---------- Заказы ------------------------------------------------------------
create table if not exists public.store_orders (
  id uuid primary key default gen_random_uuid(),
  public_id text not null unique,
  product_slug text not null,
  product_name text not null,
  category text not null,
  category_label text not null,
  nickname text not null,
  period_code text not null,
  period_label text not null,
  base_price integer not null check (base_price >= 0),
  discount_amount integer not null default 0 check (discount_amount >= 0),
  final_price integer not null check (final_price >= 0),
  currency text not null default 'RUB',
  promo_code text,
  promo_applied boolean not null default false,
  promo_message text,
  promo_percent integer,
  status text not null check (status in ('pending', 'review', 'issued', 'rejected')),
  status_label text not null,
  admin_note text,
  handled_by text,
  handled_at timestamptz,
  provider text not null default 'manual',
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now()
);

-- ---------- Индексы -----------------------------------------------------------
create index if not exists idx_store_catalog_items_sort on public.store_catalog_items(sort_order, is_active);
create index if not exists idx_store_catalog_periods_item on public.store_catalog_periods(item_id, sort_order);
create index if not exists idx_store_catalog_highlights_item on public.store_catalog_highlights(item_id, sort_order);
create index if not exists idx_store_promo_codes_active on public.store_promo_codes(is_active, code);
create index if not exists idx_store_orders_created_at on public.store_orders(created_at desc);
create index if not exists idx_store_orders_nickname on public.store_orders(nickname);
create index if not exists idx_store_orders_status on public.store_orders(status);

-- ---------- Триггеры updated_at -----------------------------------------------
drop trigger if exists trg_store_catalog_items_updated_at on public.store_catalog_items;
create trigger trg_store_catalog_items_updated_at
before update on public.store_catalog_items
for each row execute function public.set_row_updated_at();

drop trigger if exists trg_store_promo_codes_updated_at on public.store_promo_codes;
create trigger trg_store_promo_codes_updated_at
before update on public.store_promo_codes
for each row execute function public.set_row_updated_at();

drop trigger if exists trg_store_orders_updated_at on public.store_orders;
create trigger trg_store_orders_updated_at
before update on public.store_orders
for each row execute function public.set_row_updated_at();

-- ============= migrations/0002_payments.sql =============
-- =============================================================
-- 0002_payments.sql — расширяем заказы под автооплату и добавляем
-- журнал платежных транзакций. Применяется после 0001_init.sql.
-- Идемпотентна.
-- =============================================================

-- 1. Снимаем старый CHECK на provider, чтобы кроме 'manual' можно было
--    хранить yookassa и (в будущем) другие провайдеры.
do $$
declare
  con record;
begin
  for con in
    select conname
    from pg_constraint
    where conrelid = 'public.store_orders'::regclass
      and contype = 'c'
      and pg_get_constraintdef(oid) ilike '%provider%'
  loop
    execute format('alter table public.store_orders drop constraint %I', con.conname);
  end loop;
end $$;

alter table public.store_orders
  add constraint store_orders_provider_check
  check (provider in ('manual', 'yookassa'));

-- 2. Журнал платежных транзакций — нужен и для аудита, и для
--    защиты от дублей webhook'ов (UNIQUE на provider + provider_payment_id).
create table if not exists public.store_payment_transactions (
  id uuid primary key default gen_random_uuid(),
  order_public_id text not null references public.store_orders(public_id) on delete cascade,
  provider text not null check (provider in ('manual', 'yookassa')),
  provider_payment_id text not null,
  amount integer not null check (amount >= 0),
  currency text not null default 'RUB',
  status text not null check (status in ('pending', 'succeeded', 'canceled', 'refunded', 'failed')),
  raw_payload jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  unique (provider, provider_payment_id)
);

create index if not exists idx_store_payment_tx_order on public.store_payment_transactions(order_public_id);
create index if not exists idx_store_payment_tx_status on public.store_payment_transactions(status);

drop trigger if exists trg_store_payment_tx_updated_at on public.store_payment_transactions;
create trigger trg_store_payment_tx_updated_at
before update on public.store_payment_transactions
for each row execute function public.set_row_updated_at();

-- ============= migrations/0003_integrations.sql =============
-- =============================================================
-- 0003_integrations.sql — настройки исходящих webhook'ов плагина и
-- настроек платежной системы. Применяется после 0002_payments.sql.
-- Сейчас backend пишет эти данные в файловый репозиторий
-- (backend/data/integrations.json) — миграция готовит таблицы для
-- будущего перехода на Supabase, чтобы не пришлось править схему
-- "под нагрузкой". Можно применять без изменения кода: до тех пор,
-- пока StorageMode не переключён на supabase для интеграций, таблицы
-- остаются пустыми, что не мешает работе.
-- =============================================================

create table if not exists public.store_integration_endpoints (
  id text primary key check (id in ('privileges', 'cases', 'currency')),
  enabled boolean not null default false,
  url text not null default '',
  token text not null default '',
  updated_at timestamptz not null default now()
);

create table if not exists public.store_integration_secrets (
  id text primary key check (id = 'singleton'),
  webhook_secret text not null default '',
  updated_at timestamptz not null default now()
);

create table if not exists public.store_payment_settings (
  id text primary key check (id = 'singleton'),
  provider text not null default 'manual' check (provider in ('manual', 'yookassa')),
  test_mode boolean not null default false,
  shop_id text not null default '',
  secret_key text not null default '',
  public_key text not null default '',
  return_url text not null default '',
  webhook_url text not null default '',
  description text not null default '',
  updated_at timestamptz not null default now()
);

drop trigger if exists trg_store_integration_endpoints_updated_at on public.store_integration_endpoints;
create trigger trg_store_integration_endpoints_updated_at
before update on public.store_integration_endpoints
for each row execute function public.set_row_updated_at();

drop trigger if exists trg_store_integration_secrets_updated_at on public.store_integration_secrets;
create trigger trg_store_integration_secrets_updated_at
before update on public.store_integration_secrets
for each row execute function public.set_row_updated_at();

drop trigger if exists trg_store_payment_settings_updated_at on public.store_payment_settings;
create trigger trg_store_payment_settings_updated_at
before update on public.store_payment_settings
for each row execute function public.set_row_updated_at();

-- Заполняем дефолтную запись настроек, чтобы UPDATE'ы не падали.
insert into public.store_integration_endpoints (id) values ('privileges') on conflict do nothing;
insert into public.store_integration_endpoints (id) values ('cases')      on conflict do nothing;
insert into public.store_integration_endpoints (id) values ('currency')   on conflict do nothing;
insert into public.store_integration_secrets (id) values ('singleton')    on conflict do nothing;
insert into public.store_payment_settings (id, provider) values ('singleton', 'manual') on conflict do nothing;

-- ============= migrations/0004_rls.sql =============
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
