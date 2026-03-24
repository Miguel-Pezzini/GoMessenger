# TODO do Projeto

Backlog baseado no estado atual do código em `2026-03-24`.

## Prioridade Alta

### 1. Finalizar o `presence_service`

- definir o contrato de presença: `online`, `offline`, `last_seen`, `current_chat_id`
- corrigir o `services/presence_service/cmd/server.go`, que hoje está incompleto e referencia código inexistente
- adicionar o serviço ao `go.work`
- incluir o serviço no `run.go`
- decidir se presença será exposta por HTTP, gRPC ou apenas por Redis
- publicar eventos de presença em um canal próprio no Redis
- registrar conexão e desconexão a partir do `websocket_service`

### 2. Proteger o fluxo do WebSocket com identidade do JWT

- parar de confiar no `sender_id` enviado pelo cliente
- usar o `userId` autenticado para preencher o remetente no `websocket_service`
- rejeitar payloads inconsistentes

### 3. Externalizar configurações

- remover endereços hardcoded como `localhost:50051`, `localhost:27018` e `localhost:27019`
- padronizar variáveis de ambiente para todos os serviços
- documentar defaults e exemplos de `.env`

### 4. Melhorar a robustez do processamento de mensagens

- revisar o consumo do Redis Stream no `chat_service`
- avaliar uso de consumer groups em vez de `XRead` simples
- evitar perda ou reprocessamento incorreto em caso de falha no meio do fluxo

## Prioridade Média

### 5. Ampliar testes

- adicionar testes unitários para `auth_service`, `chat_service` e `websocket_service`
- cobrir cenários de token inválido e conexão sem autenticação
- testar reconexão e desconexão de WebSocket
- cobrir falhas de Redis e MongoDB
- validar o futuro fluxo de presença com testes E2E

### 6. Melhorar segurança

- mover a chave JWT para variável de ambiente
- validar melhor payloads HTTP e WebSocket
- revisar mensagens de erro e status HTTP
- considerar expiração, refresh token e rotação de segredo

### 7. Ajustar ergonomia de execução local

- criar um `docker-compose` com os serviços Go opcionais ou scripts de bootstrap
- decidir se `chat_service` deve continuar como worker puro ou expor healthcheck
- adicionar healthchecks e readiness

## Prioridade Baixa

### 8. Observabilidade

- logs estruturados
- métricas por serviço
- tracing do fluxo `gateway -> websocket_service -> redis -> chat_service`

### 9. Features de produto

- histórico de conversa por usuário
- confirmação de entrega e leitura
- lista de conversas
- paginação de mensagens
- notificações

## Pendências Técnicas Encontradas

- o `presence_service` ainda não compila no estado atual
- `tests_e2e/chat_test.go` está vazio
- `tests_e2e/main_test.go` está vazio
- o README antigo descrevia features ainda não implementadas
- o `go.work` ainda não inclui `services/presence_service`

## Sugestão de Ordem de Implementação

1. finalizar `presence_service`
2. corrigir identidade do remetente no fluxo WebSocket
3. externalizar configuração
4. reforçar testes
5. adicionar observabilidade
