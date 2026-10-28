-- cnes_equipment_types gaps (CNES honesty pass, 2026-09-01).
--
-- A real EQAC2607.dbc (DATASUS FTP, competência 2026-07) carries 139
-- distinct equipment codes; the 000202 seed (TAB_CNES/CNV/Equipam_grp.cnv)
-- names only 88 of them. The 51 others are NOT in the ministry's CNV
-- (TAB_CNES.zip fetched 2026-09-01 still lists 0101-0118, 0221-0223,
-- 0331-0350, 0441-0442, 0551-0564, 0671-0678, 0780-0786, 0887-0899,
-- 0901-0911, 1001-1004), so there is no official NAME for them and we do
-- not invent one: name = 'Código <code>' and the views render the raw code.
--
-- What CAN be sourced is the GROUP: the code is TIPEQUIP (2 digits) +
-- CODEQUIP (2 digits), and Equipamento.def binds TIPEQUIP to
-- TAB_DBF/TP_EQUIPAM.dbf ("Tipo de Equipamento"), the official group
-- table (01 Diagnóstico por Imagem … 10 Diálise). Codes below take their
-- group_name from that table so a hunt-by-name at least matches the group.
--
-- Left OUT on purpose (TIPEQUIP not in TP_EQUIPAM.dbf, no official group):
-- 1103, 1104, 1201, 1204, 1301, 1303, 1502, 1503. They still count and
-- render as "código CNES 1103" through the COALESCE in ParkByCNES.
--
-- Sources:
--   ftp://ftp.datasus.gov.br/dissemin/publicos/CNES/200508_/Auxiliar/TAB_CNES.zip
--     (CNV/Equipam_grp.cnv, DBF/TP_EQUIPAM.dbf, Equipamento.def)
--   ftp://ftp.datasus.gov.br/dissemin/publicos/CNES/200508_/Dados/EQ/EQAC2607.dbc
INSERT INTO cnes_equipment_types (code, name, group_name) VALUES
('0120', 'Código 0120', 'Equipamentos De Diagnóstico Por Imagem'),
('0121', 'Código 0121', 'Equipamentos De Diagnóstico Por Imagem'),
('0123', 'Código 0123', 'Equipamentos De Diagnóstico Por Imagem'),
('0124', 'Código 0124', 'Equipamentos De Diagnóstico Por Imagem'),
('0126', 'Código 0126', 'Equipamentos De Diagnóstico Por Imagem'),
('0127', 'Código 0127', 'Equipamentos De Diagnóstico Por Imagem'),
('0128', 'Código 0128', 'Equipamentos De Diagnóstico Por Imagem'),
('0129', 'Código 0129', 'Equipamentos De Diagnóstico Por Imagem'),
('0219', 'Código 0219', 'Equipamentos De Infra-Estrutura'),
('0220', 'Código 0220', 'Equipamentos De Infra-Estrutura'),
('0224', 'Código 0224', 'Equipamentos De Infra-Estrutura'),
('0225', 'Código 0225', 'Equipamentos De Infra-Estrutura'),
('0227', 'Código 0227', 'Equipamentos De Infra-Estrutura'),
('0228', 'Código 0228', 'Equipamentos De Infra-Estrutura'),
('0229', 'Código 0229', 'Equipamentos De Infra-Estrutura'),
('0230', 'Código 0230', 'Equipamentos De Infra-Estrutura'),
('0265', 'Código 0265', 'Equipamentos De Infra-Estrutura'),
('0266', 'Código 0266', 'Equipamentos De Infra-Estrutura'),
('0565', 'Código 0565', 'Equipamentos De Manutenção Da Vida'),
('0566', 'Código 0566', 'Equipamentos De Manutenção Da Vida'),
('0667', 'Código 0667', 'Outros Equipamentos'),
('0668', 'Código 0668', 'Outros Equipamentos'),
('0669', 'Código 0669', 'Outros Equipamentos'),
('0670', 'Código 0670', 'Outros Equipamentos'),
('0679', 'Código 0679', 'Outros Equipamentos'),
('0763', 'Código 0763', 'Equipamentos De Odontologia'),
('0765', 'Código 0765', 'Equipamentos De Odontologia'),
('0766', 'Código 0766', 'Equipamentos De Odontologia'),
('0768', 'Código 0768', 'Equipamentos De Odontologia'),
('0769', 'Código 0769', 'Equipamentos De Odontologia'),
('0772', 'Código 0772', 'Equipamentos De Odontologia'),
('0773', 'Código 0773', 'Equipamentos De Odontologia'),
('0775', 'Código 0775', 'Equipamentos De Odontologia'),
('0776', 'Código 0776', 'Equipamentos De Odontologia'),
('0777', 'Código 0777', 'Equipamentos De Odontologia'),
('0778', 'Código 0778', 'Equipamentos De Odontologia'),
('0779', 'Código 0779', 'Equipamentos De Odontologia'),
('0787', 'Código 0787', 'Equipamentos De Odontologia'),
('0788', 'Código 0788', 'Equipamentos De Odontologia'),
('0793', 'Código 0793', 'Equipamentos De Odontologia'),
('0795', 'Código 0795', 'Equipamentos De Odontologia'),
('0799', 'Código 0799', 'Equipamentos De Odontologia'),
('0912', 'Código 0912', 'Equipamentos De Telessaúde')
ON CONFLICT (code) DO NOTHING;
