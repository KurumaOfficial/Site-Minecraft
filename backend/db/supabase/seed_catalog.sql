-- Part 1: Insert catalog items
insert into store_catalog_items (external_id, slug, name, category, category_label, summary, image, currency, sort_order, is_active)
values
  ('privilege-vip', 'vip', 'VIP', 'privilege', 'Привилегия', 'Базовая постоянная привилегия сервера ESTELAR.', 'assets/images/image-05-3574266306.png', 'RUB', 1, true),
  ('privilege-crystal', 'crystal', 'CRYSYSTAL', 'privilege', 'Привилегия', 'Усиленная постоянная привилегия сервера ESTELAR с фиксированной ценой.', 'assets/images/image-06-ec43498384.png', 'RUB', 2, true),
  ('privilege-dragon', 'dragon', 'GALAXY', 'privilege', 'Привилегия', 'Продвинутая постоянная привилегия ESTELAR для активных игроков.', 'assets/images/image-07-524845b051.png', 'RUB', 3, true),
  ('privilege-king', 'king', 'KING', 'privilege', 'Привилегия', 'Постоянная привилегия повышенного уровня для сервера ESTELAR.', 'assets/images/image-08-cb0961288a.png', 'RUB', 4, true),
  ('privilege-titan', 'titan', 'TITAN', 'privilege', 'Привилегия', 'Мощная постоянная привилегия для основной аудитории доната ESTELAR.', 'assets/images/image-09-b8a9706aa0.png', 'RUB', 5, true),
  ('privilege-phenix', 'phenix', 'PHENIX', 'privilege', 'Привилегия', 'Редкая постоянная привилегия премиум-уровня ESTELAR.', 'assets/images/image-10-597bf172e7.png', 'RUB', 6, true),
  ('privilege-immortal', 'immortal', 'IMMORTAL', 'privilege', 'Привилегия', 'Топовая постоянная привилегия ESTELAR с высоким статусом.', 'assets/images/image-11-cd3fc52735.png', 'RUB', 7, true),
  ('privilege-infinity', 'infinity', 'INFINITY', 'privilege', 'Привилегия', 'Максимальная постоянная привилегия для игроков, которым нужен полный доступ к донат-линейке ESTELAR.SU.', 'assets/images/image-12-e7d68a881c.png', 'RUB', 8, true),
  ('service-unmute', 'unmute', 'РАЗМУТ', 'privilege', 'Услуга', 'РАЗМУТ НА СЕРВЕРЕ
При покупке размута аккаунт который вы указали в корзине размутится.', 'assets/images/image-15-e2262cce74.png', 'RUB', 9, true),
  ('service-unban', 'unban', 'РАЗБАН', 'privilege', 'Услуга', 'РАЗБАН НА СЕРВЕРЕ
При покупке разбана аккаунт который вы указали в корзине разбанится.', 'assets/images/image-14-743744227c.png', 'RUB', 10, true),
  ('service-immunity', 'immunity', 'ИММУНИТЕТ', 'privilege', 'Услуга', 'ИМУНИТЕТ ОТ БАНА на неделю и только от игроков.
При покупке иммунитета аккаунт который вы указали в корзине будет невозможно забанить игрокам, но не админам.', 'assets/images/image-13-ac5211f93c.png', 'RUB', 11, true),
  ('currency-donate', 'donate-currency', '100 EST', 'case', 'Валюта', '100 EST — донат-валюта сервера ESTELAR.SU.
После покупки на аккаунт поступает 100 EST за каждый выбранный пакет.', 'assets/images/image-13-ac5211f93c.png', 'RUB', 12, true)
on conflict (slug) do nothing;

-- Part 2: Insert periods
insert into store_catalog_periods (item_id, code, label, price, is_default, sort_order)
select sci.id, prices.code, prices.label, prices.price_value, true, 1
from (
  values
    ('vip', 'forever', 'Навсегда', 19),
    ('crystal', 'forever', 'Навсегда', 49),
    ('dragon', 'forever', 'Навсегда', 99),
    ('king', 'forever', 'Навсегда', 179),
    ('unmute', 'forever', 'Навсегда', 99),
    ('unban', 'forever', 'Навсегда', 199),
    ('immunity', 'forever', 'Навсегда', 499),
    ('titan', 'forever', 'Навсегда', 249),
    ('phenix', 'forever', 'Навсегда', 369),
    ('immortal', 'forever', 'Навсегда', 499),
    ('infinity', 'forever', 'Навсегда', 699),
    ('donate-currency', 'units', '1 пакет = 100 EST', 99)
) as prices(slug, code, label, price_value)
join store_catalog_items sci on sci.slug = prices.slug
on conflict (item_id, code) do nothing;

-- Part 3: Insert highlights
insert into store_catalog_highlights (item_id, sort_order, text)
select sci.id, h.sort_order, h.text
from store_catalog_items sci
join (
  values
    ('vip', 1, 'Навсегда без продления'),
    ('vip', 2, 'Стартовый уровень магазина'),
    ('vip', 3, 'Фиксированная цена: 19 руб.'),
    ('crystal', 1, 'Навсегда без продления'),
    ('crystal', 2, 'Второй уровень привилегий'),
    ('crystal', 3, 'Фиксированная цена: 49 руб.'),
    ('dragon', 1, 'Навсегда без продления'),
    ('dragon', 2, 'Продвинутая линейка магазина'),
    ('dragon', 3, 'Фиксированная цена: 99 руб.'),
    ('king', 1, 'Навсегда без продления'),
    ('king', 2, 'Повышенный уровень статуса'),
    ('king', 3, 'Фиксированная цена: 179 руб.'),
    ('titan', 1, 'Навсегда без продления'),
    ('titan', 2, 'Старший уровень магазина'),
    ('titan', 3, 'Фиксированная цена: 249 руб.'),
    ('phenix', 1, 'Навсегда без продления'),
    ('phenix', 2, 'Премиальная линейка'),
    ('phenix', 3, 'Фиксированная цена: 369 руб.'),
    ('immortal', 1, 'Навсегда без продления'),
    ('immortal', 2, 'Почти максимальный уровень'),
    ('immortal', 3, 'Фиксированная цена: 499 руб.'),
    ('infinity', 1, 'Навсегда без продления'),
    ('infinity', 2, 'Максимальный уровень линейки'),
    ('infinity', 3, 'Фиксированная цена: 699 руб.'),
    ('unmute', 1, 'Разовая услуга'),
    ('unmute', 2, 'Ручная обработка заявки'),
    ('unmute', 3, 'Стоимость: 99 руб.'),
    ('unban', 1, 'Разовая услуга'),
    ('unban', 2, 'Ручная обработка заявки'),
    ('unban', 3, 'Стоимость: 199 руб.'),
    ('immunity', 1, 'Срок действия: 1 неделя'),
    ('immunity', 2, 'Работает только против игроков'),
    ('immunity', 3, 'Администрация сохраняет полный доступ к наказаниям'),
    ('donate-currency', 1, 'EST — внутриигровая донат-валюта'),
    ('donate-currency', 2, '1 пакет = 100 EST'),
    ('donate-currency', 3, 'Стоимость пакета: 99 руб.')
) as h(slug, sort_order, text) on sci.slug = h.slug
on conflict do nothing;

-- Part 4: Insert promo codes
insert into store_promo_codes (code, discount_percent, is_active)
values ('EST15', 15, true)
on conflict (code) do update
set discount_percent = excluded.discount_percent,
    is_active = excluded.is_active,
    updated_at = now();
