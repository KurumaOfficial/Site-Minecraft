-- Автор: Kuruma
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
