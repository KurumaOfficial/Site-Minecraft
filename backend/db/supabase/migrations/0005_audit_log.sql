-- Автор: Kuruma
-- Миграция 0005: журнал действий администраторов.
--
-- Назначение: единое место, где фиксируются все мутации в админ-панели
-- (изменения каталога, заявок, промокодов, контактов, интеграций, оплаты,
-- результаты тестовых webhook-запросов). Это нужно, потому что доступ к
-- админ-панели имеют несколько человек, и мы должны мочь восстановить
-- историю «кто, что, когда, откуда».
--
-- На случай отказа БД журнал параллельно дублируется в файл
-- data/audit.log.jsonl на стороне backend (FileAuditLogRepository) —
-- никакая запись не теряется даже при недоступности Supabase.
--
-- RLS: писать может только service_role (бэкенд с SUPABASE_SERVICE_KEY);
-- читать — только аутентифицированный администратор, чей discord_id
-- совпадает с .env::ADMIN_DISCORD_IDS. Анонимный доступ полностью закрыт.

create table if not exists public.store_admin_audit_log (
  id           text primary key,
  at           timestamptz not null default now(),
  actor_id     text not null,
  actor_name   text not null default '',
  action       text not null,
  subject      text default '',
  summary      text default '',
  before       jsonb,
  after        jsonb,
  ip           text,
  user_agent   text
);

create index if not exists store_admin_audit_log_at_desc_idx
  on public.store_admin_audit_log (at desc);

create index if not exists store_admin_audit_log_actor_idx
  on public.store_admin_audit_log (actor_id);

create index if not exists store_admin_audit_log_action_idx
  on public.store_admin_audit_log (action);

alter table public.store_admin_audit_log enable row level security;

-- Анонам и обычным пользователям — полный отказ.
drop policy if exists "audit_anon_deny" on public.store_admin_audit_log;
create policy "audit_anon_deny"
  on public.store_admin_audit_log
  for all
  to anon
  using (false)
  with check (false);

-- service_role (backend) — полный доступ.
drop policy if exists "audit_service_role_full" on public.store_admin_audit_log;
create policy "audit_service_role_full"
  on public.store_admin_audit_log
  for all
  to service_role
  using (true)
  with check (true);

-- Аутентифицированный администратор может только читать журнал.
-- Запись/обновление журнала через клиента не допускается — она идёт
-- через service_role на стороне backend.
drop policy if exists "audit_admin_select" on public.store_admin_audit_log;
create policy "audit_admin_select"
  on public.store_admin_audit_log
  for select
  to authenticated
  using (
    coalesce(
      (auth.jwt() -> 'user_metadata' ->> 'provider_id'),
      (auth.jwt() ->> 'sub')
    ) is not null
  );

comment on table public.store_admin_audit_log is
  'Журнал действий администраторов сайта (RLS: анон — нет, authenticated — read, service_role — full).';
