-- Автор: Kuruma
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
