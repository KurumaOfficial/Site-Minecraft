-- Автор: Kuruma
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
