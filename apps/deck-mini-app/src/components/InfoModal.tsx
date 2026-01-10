import { useState } from 'react'

interface InfoModalProps {
  isOpen: boolean
  onClose: () => void
}

type Tab = 'criteria' | 'technical'

/**
 * Modal overlay displaying scoring criteria and technical details.
 */
export function InfoModal({ isOpen, onClose }: InfoModalProps) {
  const [activeTab, setActiveTab] = useState<Tab>('criteria')

  if (!isOpen) return null

  return (
    <div className="info-modal-overlay" onClick={onClose}>
      <div className="info-modal" onClick={(e) => e.stopPropagation()}>
        <div className="info-modal-header">
          <div className="info-tabs">
            <button
              className={`info-tab ${activeTab === 'criteria' ? 'active' : ''}`}
              onClick={() => setActiveTab('criteria')}
            >
              Como funciona
            </button>
            <button
              className={`info-tab ${activeTab === 'technical' ? 'active' : ''}`}
              onClick={() => setActiveTab('technical')}
            >
              Técnico
            </button>
          </div>
          <button className="info-close-btn" onClick={onClose}>
            ✕
          </button>
        </div>

        <div className="info-modal-content">
          {activeTab === 'criteria' ? <CriteriaContent /> : <TechnicalContent />}
        </div>
      </div>
    </div>
  )
}

function CriteriaContent() {
  return (
    <div className="info-content">
      <h2>Como seu card é calculado</h2>
      <p>
        Seu card semanal mede sua participação no grupo usando 7 métricas. Cada
        uma analisa um aspecto diferente da sua atividade.
      </p>

      <h3>Como o Score Geral é Calculado</h3>

      <h4>Contribuição positiva:</h4>
      <ul>
        <li>Popularidade: 20%</li>
        <li>Presença: 15%</li>
        <li>Aura: 12%</li>
        <li>Sequência: 10%</li>
        <li>Humor: 8%</li>
        <li>Atividade: 5%</li>
      </ul>

      <h4>Penalidades:</h4>
      <ul className="penalties">
        <li>Toxicidade: -12%</li>
        <li>Reações negativas: -7%</li>
        <li>Mensagens negativas: -6%</li>
        <li>Maior intervalo: -5%</li>
      </ul>

      <h3>Modificadores</h3>

      <h4>Tendência semanal</h4>
      <p className="modifier-desc">Comparando com a semana anterior:</p>
      <ul>
        <li>Melhora significativa: +5 pontos</li>
        <li>Pequena melhora: +3 pontos</li>
        <li>Pequena piora: -3 pontos</li>
        <li>Piora significativa: -5 pontos</li>
      </ul>

      <h4>Badges</h4>
      <p className="modifier-desc">Conquistas especiais:</p>
      <ul>
        <li>Lendários: +10 pontos</li>
        <li>Épicos: +7 pontos</li>
        <li>Comuns: +3 pontos</li>
        <li>Negativos: -5 pontos</li>
      </ul>

      <h3>Badges Positivos</h3>
      <ul className="badges-list">
        <li>
          <span className="badge-icon">🌟</span>
          <span className="badge-name">Radiant</span>
          <span className="badge-desc">Aura &gt;= 80 (Lendário)</span>
        </li>
        <li>
          <span className="badge-icon">🔥</span>
          <span className="badge-name">Hyperactive</span>
          <span className="badge-desc">500+ mensagens (Lendário)</span>
        </li>
        <li>
          <span className="badge-icon">📅</span>
          <span className="badge-name">Regular</span>
          <span className="badge-desc">Presença &gt;= 80 (Épico)</span>
        </li>
        <li>
          <span className="badge-icon">🎭</span>
          <span className="badge-name">Comedian</span>
          <span className="badge-desc">Humor &gt;= 70 (Lendário)</span>
        </li>
        <li>
          <span className="badge-icon">☮️</span>
          <span className="badge-name">Zen</span>
          <span className="badge-desc">Toxicidade &lt; 2% (Lendário)</span>
        </li>
        <li>
          <span className="badge-icon">⭐</span>
          <span className="badge-name">Star</span>
          <span className="badge-desc">10+ reatores únicos (Lendário)</span>
        </li>
        <li>
          <span className="badge-icon">📈</span>
          <span className="badge-name">N-day streak</span>
          <span className="badge-desc">7+ dias consecutivos (Comum)</span>
        </li>
      </ul>

      <h3>Badges Negativos</h3>
      <ul className="badges-list negative">
        <li>
          <span className="badge-icon">🌧️</span>
          <span className="badge-name">Gloomy</span>
          <span className="badge-desc">Aura &lt; 30</span>
        </li>
        <li>
          <span className="badge-icon">👻</span>
          <span className="badge-name">Ghost</span>
          <span className="badge-desc">&lt; 20 mensagens</span>
        </li>
        <li>
          <span className="badge-icon">🌍</span>
          <span className="badge-name">Tourist</span>
          <span className="badge-desc">Presença &lt; 20 (com 10+ msgs)</span>
        </li>
        <li>
          <span className="badge-icon">😐</span>
          <span className="badge-name">Deadpan</span>
          <span className="badge-desc">Humor &lt; 10</span>
        </li>
        <li>
          <span className="badge-icon">☠️</span>
          <span className="badge-name">Toxic</span>
          <span className="badge-desc">Toxicidade &gt; 20%</span>
        </li>
        <li>
          <span className="badge-icon">🦗</span>
          <span className="badge-name">Cricket</span>
          <span className="badge-desc">0 reações (com 30+ msgs)</span>
        </li>
      </ul>

      <h3>Tiers (Níveis)</h3>
      <p>Seu tier é definido pelo score geral:</p>
      <ul className="tiers-list">
        <li>
          <span className="tier-score">81+</span> Lendário
        </li>
        <li>
          <span className="tier-score">77+</span> Bichão
        </li>
        <li>
          <span className="tier-score">72+</span> CLT
        </li>
        <li>
          <span className="tier-score">55+</span> Coadjuvante
        </li>
        <li>
          <span className="tier-score">32+</span> Fióti
        </li>
        <li>
          <span className="tier-score">10+</span> Random
        </li>
      </ul>

      <h3>As 7 Métricas</h3>

      <div className="metric-card">
        <h4>Aura</h4>
        <p>Mede o tom das suas mensagens e como os outros reagem a você</p>
        <ul>
          <li>Mensagens positivas, neutras e negativas</li>
          <li>Reações positivas recebidas</li>
        </ul>
      </div>

      <div className="metric-card">
        <h4>Atividade</h4>
        <p>Volume de contribuição</p>
        <ul>
          <li>Quantidade de mensagens</li>
          <li>Tamanho médio das mensagens</li>
          <li>Reações e respostas enviadas</li>
        </ul>
      </div>

      <div className="metric-card">
        <h4>Presença</h4>
        <p>Consistência vs aparições esporádicas</p>
        <ul>
          <li>Dias ativos no período</li>
          <li>Sequência de dias consecutivos</li>
          <li>Variedade de horários</li>
        </ul>
      </div>

      <div className="metric-card">
        <h4>Humor</h4>
        <p>Impacto cômico</p>
        <ul>
          <li>Reações positivas em mensagens suas</li>
          <li>Usuários únicos que reagiram</li>
          <li>Mensagens classificadas como humor</li>
        </ul>
      </div>

      <div className="metric-card">
        <h4>Toxicidade</h4>
        <p>Impacto negativo (penalidade)</p>
        <ul>
          <li>Mensagens tóxicas detectadas</li>
          <li>Reações negativas recebidas</li>
        </ul>
      </div>

      <div className="metric-card">
        <h4>Popularidade</h4>
        <p>Atração social</p>
        <ul>
          <li>Usuários únicos que reagiram</li>
          <li>Usuários únicos que responderam</li>
          <li>Mensagens virais (4+ reações)</li>
        </ul>
      </div>

      <h3>Estatísticas de Combate</h3>
      <p>
        Além das métricas, seu card inclui stats de combate derivadas para
        futuras funcionalidades de jogo:
      </p>

      <div className="metric-card">
        <h4>⚔️ ATK (1-10)</h4>
        <p>Poder ofensivo</p>
        <ul>
          <li>40% Atividade</li>
          <li>35% Toxicidade</li>
          <li>25% Humor</li>
        </ul>
      </div>

      <div className="metric-card">
        <h4>🛡️ DEF (1-10)</h4>
        <p>Capacidade defensiva</p>
        <ul>
          <li>40% Presença</li>
          <li>35% Aura</li>
          <li>25% Popularidade</li>
        </ul>
      </div>

      <div className="metric-card">
        <h4>🥩 HP (3-30)</h4>
        <p>Pontos de vida</p>
        <ul>
          <li>DEF × 3</li>
        </ul>
      </div>

      <p className="note">
        Essas estatísticas não afetam seu Score Geral ou tier.
      </p>
    </div>
  )
}

function TechnicalContent() {
  return (
    <div className="info-content">
      <h2>Detalhes Técnicos</h2>
      <p>
        Informações sobre os modelos e algoritmos usados para gerar seu card.
      </p>

      <h3>Modelos de ML</h3>

      <div className="tech-card">
        <h4>Análise de Sentimento</h4>
        <code>distilbert-base-multilingual-cased-sentiments-student</code>
        <ul>
          <li>Classifica mensagens como positiva, neutra ou negativa</li>
          <li>Suporta 6 idiomas incluindo português</li>
          <li>Framework: HuggingFace Transformers</li>
        </ul>
      </div>

      <div className="tech-card">
        <h4>Detecção de Toxicidade</h4>
        <code>bert-base-portuguese-cased-hatebr</code>
        <ul>
          <li>Treinado no corpus HateBR de discurso de ódio brasileiro</li>
          <li>Detecta mensagens com conteúdo tóxico ou ofensivo</li>
        </ul>
      </div>

      <div className="tech-card">
        <h4>Detecção de Humor</h4>
        <p className="method-label">Método: Heurísticas baseadas em padrões</p>
        <ul>
          <li>Detecta risadas</li>
          <li>Analisa emojis de risada</li>
          <li>Threshold: score &gt;= 0.4 para classificar como humor</li>
        </ul>
      </div>

      <div className="tech-card">
        <h4>Sentimento de Emojis</h4>
        <p className="method-label">Biblioteca: emosent-py</p>
        <ul>
          <li>Base de 751 emojis com scores de sentimento</li>
          <li>Positivo: score &gt; 0.2</li>
          <li>Negativo: score &lt; -0.2</li>
        </ul>
      </div>

      <h3>Normalização Estatística</h3>

      <div className="tech-card">
        <h4>Bayesian Smoothing Progressivo</h4>
        <p>Todos os scores usam suavização Bayesiana com k progressivo:</p>
        <div className="formula">
          <code>k = 5 + 25 × (0.5 ^ (n / 20))</code>
          <code>score = (n * raw + k * média) / (n + k)</code>
        </div>
        <p>O valor de k diminui conforme aumenta o número de mensagens:</p>
        <ul className="k-values">
          <li>
            <span>1 msg</span> <span>k≈29 (forte suavização)</span>
          </li>
          <li>
            <span>20 msgs</span> <span>k≈18 (moderada)</span>
          </li>
          <li>
            <span>50 msgs</span> <span>k≈9 (leve)</span>
          </li>
          <li>
            <span>100 msgs</span> <span>k≈6 (mínima)</span>
          </li>
        </ul>
        <p className="note">
          Isso protege usuários com poucas mensagens de scores extremos,
          enquanto permite que usuários ativos alcancem seus scores reais.
        </p>
      </div>

      <div className="tech-card">
        <h4>Normalização p90</h4>
        <p>
          Métricas de volume (mensagens, reações) são normalizadas pelo
          percentil 90 do chat, evitando que outliers distorçam os resultados.
        </p>
      </div>

      <h3>Janela de Dados</h3>
      <ul>
        <li>Período: 30 dias (janela deslizante)</li>
        <li>Cards: Gerados semanalmente (segunda a domingo)</li>
        <li>Mínimo: 10 mensagens para aparecer no ranking</li>
      </ul>

      <h3>Estatísticas de Combate</h3>
      <p>Derivadas das métricas existentes para gamificação:</p>

      <div className="tech-card">
        <h4>⚔️ ATK (Ataque)</h4>
        <div className="formula">
          <code>raw_atk = 0.40×Atividade + 0.35×Toxicidade + 0.25×Humor</code>
          <code>ATK = max(1, round(raw_atk / 10))</code>
        </div>
      </div>

      <div className="tech-card">
        <h4>🛡️ DEF (Defesa)</h4>
        <div className="formula">
          <code>raw_def = 0.40×Presença + 0.35×Aura + 0.25×Popularidade</code>
          <code>DEF = max(1, round(raw_def / 10))</code>
        </div>
      </div>

      <div className="tech-card">
        <h4>🥩 HP (Vida)</h4>
        <div className="formula">
          <code>HP = DEF × 3</code>
        </div>
      </div>

      <p>Range: ATK/DEF (1-10), HP (3-30)</p>
    </div>
  )
}
