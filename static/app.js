// Common tool logic extraction for reusability
async function performToolRequest(endpoint, payload, resultElement, buttonElement, loadingMessage, successCallback) {
  const input = payload.text || payload.json;
  if (!input || !input.trim()) {
    resultElement.textContent = 'Please enter some text to process.';
    resultElement.className = 'result error';
    return;
  }

  resultElement.textContent = loadingMessage;
  resultElement.className = 'result loading';
  buttonElement.disabled = true;

  try {
    const res = await fetch(endpoint, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'Accept': 'application/json'
      },
      body: JSON.stringify(payload)
    });

    if (!res.ok) {
      throw new Error(`Server error: ${res.status}`);
    }

    const contentType = res.headers.get('content-type');
    if (!contentType || !contentType.includes('application/json')) {
      throw new Error('Invalid response format');
    }

    const data = await res.json();
    successCallback(data, resultElement);
    resultElement.className = 'result success';
  } catch (err) {
    console.error('Network error:', err);
    // Safari-specific error handling using feature detection
    if (isSafari && err.message.includes('Invalid response format')) {
        resultElement.textContent = 'Error: Safari compatibility issue. Please try again.';
    } else {
        resultElement.textContent = `Error: ${err.message}`;
    }
    resultElement.className = 'result error';
  } finally {
    buttonElement.disabled = false;
  }
}

// Token Counter
const countBtn = document.getElementById('countBtn');
const tokenInput = document.getElementById('tokenInput');
const tokenResult = document.getElementById('tokenResult');

countBtn.onclick = () => {
  performToolRequest(
    '/api/count-tokens',
    { text: tokenInput.value.trim() },
    tokenResult,
    countBtn,
    'Counting tokens...',
    (data, resultElement) => {
      // Create DOM elements instead of using innerHTML with inline styles
      const grid = document.createElement('div');
      grid.className = 'token-grid';

      // Helper function to create stat elements
      const createStat = (value, label, className) => {
        const stat = document.createElement('div');
        stat.className = `token-stat ${className}`;

        const valueDiv = document.createElement('div');
        valueDiv.className = 'stat-value';
        valueDiv.textContent = value || 0;

        const labelDiv = document.createElement('div');
        labelDiv.className = 'stat-label';
        labelDiv.textContent = label;

        stat.appendChild(valueDiv);
        stat.appendChild(labelDiv);
        return stat;
      };

      grid.appendChild(createStat(data.tokens, 'Tokens', 'tokens'));
      grid.appendChild(createStat(data.words, 'Words', 'words'));
      grid.appendChild(createStat(data.characters, 'Characters (no spaces)', 'characters'));
      grid.appendChild(createStat(data.charactersWithSpaces, 'Characters (with spaces)', 'characters-spaces'));

      // Create pro tip
      const tip = document.createElement('div');
      tip.className = 'pro-tip';
      tip.innerHTML = '<span class="emoji-decorative" aria-hidden="true">💡</span> <strong>Pro tip:</strong> Use TOON format to reduce token usage by up to 60%!';

      // Clear and append new content
      resultElement.innerHTML = '';
      resultElement.appendChild(grid);
      resultElement.appendChild(tip);
    }
  );
};

// JSON Fixer
const fixBtn = document.getElementById('fixBtn');
const fixInput = document.getElementById('fixInput');
const fixResult = document.getElementById('fixResult');
const copyFixBtn = document.getElementById('copyFixBtn');

let currentFixData = '';

fixBtn.onclick = () => {
  performToolRequest(
    '/api/fix-json',
    { json: fixInput.value.trim() },
    fixResult,
    fixBtn,
    '🔧 Fixing JSON...',
    (data, resultElement) => {
      if (data.error) {
        const errorSpan = document.createElement('span');
        errorSpan.className = 'error-text';
        errorSpan.innerHTML = `<span class="emoji-decorative" aria-hidden="true">❌</span> Error: ${escapeHtml(data.error)}`;
        resultElement.appendChild(errorSpan);
        copyFixBtn.disabled = true;
        currentFixData = '';
      } else if (data.fixed) {
        currentFixData = data.fixed;

        // Success message
        const successMsg = document.createElement('div');
        successMsg.className = 'success-message';
        successMsg.innerHTML = '<span class="emoji-decorative" aria-hidden="true">✅</span> JSON fixed successfully!';
        resultElement.appendChild(successMsg);

        // Changes list if available
        if (data.changes && data.changes.length > 0) {
          const changesList = document.createElement('div');
          changesList.className = 'changes-list';
          changesList.innerHTML = '<strong>Changes made:</strong><ul>' +
            data.changes.map(change => `<li><span class="emoji-decorative" aria-hidden="true">🔧</span> ${escapeHtml(change)}</li>`).join('') +
            '</ul>';
          resultElement.appendChild(changesList);
        }

        // Fixed JSON output
        const jsonPre = document.createElement('pre');
        jsonPre.className = 'json-output';
        jsonPre.textContent = data.fixed; // Use textContent for pre-formatted text
        resultElement.appendChild(jsonPre);

        // Tip message
        const tip = document.createElement('div');
        tip.className = 'pro-tip';
        tip.innerHTML = '<span class="emoji-decorative" aria-hidden="true">💡</span> <strong>Tip:</strong> Now convert this JSON to TOON format to save even more tokens!';
        resultElement.appendChild(tip);

        copyFixBtn.disabled = false;
      } else {
        throw new Error('Invalid server response');
      }
    }
  );
};

copyFixBtn.onclick = () => {
  if (copyFixBtn.disabled || !currentFixData) return;

  navigator.clipboard.writeText(currentFixData)
    .then(() => {
      const originalText = copyFixBtn.textContent;
      copyFixBtn.textContent = '✓ Copied!';
      copyFixBtn.classList.add('copied');
      copyFixBtn.setAttribute('aria-label', 'Copied to clipboard');

      setTimeout(() => {
        copyFixBtn.textContent = originalText;
        copyFixBtn.classList.remove('copied');
        copyFixBtn.setAttribute('aria-label', 'Copy fixed JSON to clipboard');
      }, 1500);
    })
    .catch((err) => {
      copyFixBtn.textContent = '✗ Copy failed';
      copyFixBtn.setAttribute('aria-label', 'Copy failed');
      setTimeout(() => {
        copyFixBtn.textContent = 'Copy Result';
        copyFixBtn.setAttribute('aria-label', 'Copy fixed JSON to clipboard');
      }, 1500);
    });
};

// JSON to TOON Converter
// TOON (Token-Oriented Object Notation) is a compact data format designed for Large Language Models.
// It reduces token usage by 30-60% compared to JSON while maintaining readability and structure.
// Key optimizations: removes redundant punctuation, uses indentation for structure, and tabularizes repeated data.
const convertBtn = document.getElementById('convertBtn');
const jsonInput = document.getElementById('jsonInput');
const toonResult = document.getElementById('toonResult');
const copyToonBtn = document.getElementById('copyToonBtn');

// Variable para guardar el TOON real
let currentToonData = '';

convertBtn.onclick = () => {
  performToolRequest(
    '/api/json-to-toon',
    { json: jsonInput.value.trim() },
    toonResult,
    convertBtn,
    '🔄 Converting to TOON...',
    (data, resultElement) => {
      if (data.error && !data.toon) {
        // Error without solution
        const errorSpan = document.createElement('span');
        errorSpan.className = 'error-text';
        errorSpan.innerHTML = `<span class="emoji-decorative" aria-hidden="true">❌</span> Error: ${escapeHtml(data.error)}`;
        resultElement.appendChild(errorSpan);
        copyToonBtn.disabled = true;
        currentToonData = '';
      } else if (data.toon) {
        // Success (with or without correction)
        currentToonData = data.toon;

        // Show warning if it was corrected
        if (data.fixed) {
          const warningMsg = document.createElement('div');
          warningMsg.className = 'warning-message';
          warningMsg.innerHTML = `<span class="emoji-decorative" aria-hidden="true">⚠️</span> ${escapeHtml(data.error)}`;
          resultElement.appendChild(warningMsg);
          resultElement.className = 'result warning';
        } else {
          resultElement.className = 'result success';
        }

        // Show token comparison if available
        if (data.tokenSavings) {
          const savings = data.tokenSavings;

          const savingsBanner = document.createElement('div');
          savingsBanner.className = 'token-savings-banner';

          const icon = document.createElement('div');
          icon.className = 'savings-icon emoji-decorative';
          icon.setAttribute('aria-hidden', 'true');
          icon.textContent = '💰';
          savingsBanner.appendChild(icon);

          const title = document.createElement('div');
          title.className = 'savings-title';
          title.textContent = `${savings.saved} Tokens Saved!`;
          savingsBanner.appendChild(title);

          const percentage = document.createElement('div');
          percentage.className = 'savings-percentage';
          percentage.textContent = `${savings.percentage}% Reduction`;
          savingsBanner.appendChild(percentage);

          const comparison = document.createElement('div');
          comparison.className = 'token-savings-comparison';

          const jsonItem = document.createElement('div');
          jsonItem.className = 'token-savings-item';
          jsonItem.innerHTML = `<strong>JSON:</strong> ${savings.json} tokens`;
          comparison.appendChild(jsonItem);

          const toonItem = document.createElement('div');
          toonItem.className = 'token-savings-item';
          toonItem.innerHTML = `<strong>TOON:</strong> ${savings.toon} tokens`;
          comparison.appendChild(toonItem);

          savingsBanner.appendChild(comparison);
          resultElement.appendChild(savingsBanner);
        }

        // Show the TOON
        const toonPre = document.createElement('pre');
        toonPre.className = 'toon-output';
        toonPre.textContent = data.toon; // Use textContent for pre-formatted text, no need to escape since it's code
        resultElement.appendChild(toonPre);

        // Success message
        const successMsg = document.createElement('div');
        successMsg.className = 'pro-tip';
        successMsg.innerHTML = '<span class="emoji-decorative" aria-hidden="true">🎉</span> <strong>Success!</strong> Your TOON format is ready to use with AI models!';
        resultElement.appendChild(successMsg);

        copyToonBtn.disabled = false;
      } else {
        throw new Error('Invalid server response');
      }
    }
  );
};

copyToonBtn.onclick = () => {
  if (copyToonBtn.disabled || !currentToonData) return;

  navigator.clipboard.writeText(currentToonData)
    .then(() => {
      const originalText = copyToonBtn.textContent;
      copyToonBtn.textContent = '✓ Copied!';
      copyToonBtn.classList.add('copied');
      copyToonBtn.setAttribute('aria-label', 'Copied to clipboard');

      setTimeout(() => {
        copyToonBtn.textContent = originalText;
        copyToonBtn.classList.remove('copied');
        copyToonBtn.setAttribute('aria-label', 'Copy TOON result to clipboard');
      }, 1500);
    })
    .catch((err) => {
      copyToonBtn.textContent = '✗ Copy failed';
      copyToonBtn.setAttribute('aria-label', 'Copy failed');
      setTimeout(() => {
        copyToonBtn.textContent = 'Copy Result';
        copyToonBtn.setAttribute('aria-label', 'Copy TOON result to clipboard');
      }, 1500);
    });
};

// Función helper para escapar HTML - ensures safe rendering of user-generated content
function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// Modal functions with accessibility improvements
function openModal(modalType) {
  const modal = document.getElementById(modalType + '-modal');
  modal.style.display = 'block';
  modal.setAttribute('aria-hidden', 'false');
  document.body.style.overflow = 'hidden';

  // Focus trapping and keyboard navigation
  const focusableElements = modal.querySelectorAll('button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])');
  const firstFocusable = focusableElements[0];
  const lastFocusable = focusableElements[focusableElements.length - 1];

  if (firstFocusable) {
    firstFocusable.focus();
  }

  modal.addEventListener('keydown', function(e) {
    if (e.key === 'Escape') {
      closeModal(modal.id);
    }

    if (e.key === 'Tab') {
      if (e.shiftKey) {
        if (document.activeElement === firstFocusable) {
          lastFocusable.focus();
          e.preventDefault();
        }
      } else {
        if (document.activeElement === lastFocusable) {
          firstFocusable.focus();
          e.preventDefault();
        }
      }
    }
  });
}

function closeModal(modalId) {
  const modal = document.getElementById(modalId);
  modal.style.display = 'none';
  modal.setAttribute('aria-hidden', 'true');
  document.body.style.overflow = 'auto';
}

// Close modal when clicking outside
window.onclick = function(event) {
  if (event.target.classList.contains('modal')) {
    event.target.style.display = 'none';
    document.body.style.overflow = 'auto';
  }
}

// Language toggle functionality
let currentLanguage = 'en';

const translations = {
  en: {
    title: "JSON Token Optimizer - Reduce LLM Tokens by 60% | Free Online Tool",
    heroTitle: "JSON Token Optimizer - Reduce LLM API Costs by 60%",
    heroSubtitle: "Free online tool to convert JSON to TOON format, count tokens, and fix malformed JSON. Optimize your data for ChatGPT, Claude, GPT-4, and other Large Language Models.",
    toolsHeader: "🛠️ JSON Token Optimizer Tools",
    toolsSubtitle: "Choose a tool below to optimize your JSON for AI models",
    tokenCounterTitle: "Token Counter",
    tokenCounterDesc: "Count tokens in your text before sending to AI APIs",
    jsonRepairTitle: "JSON Repair Tool",
    jsonRepairDesc: "Automatically fix broken or malformed JSON",
    toonConverterTitle: "TOON Converter",
    toonConverterDesc: "Convert JSON to TOON format and save up to 60% on tokens",
    featuresTitle: "Why Use JSON Token Optimizer?",
    howItWorksTitle: "How to Optimize JSON for Large Language Models",
    comparisonTitle: "JSON vs TOON: Real Token Savings",
    useCasesTitle: "Who Benefits from JSON Token Optimization?",
    faqTitle: "Frequently Asked Questions",
    footerTitle: "JSON Token Optimizer"
  },
  es: {
    title: "Optimizador de Tokens JSON - Reduce Tokens LLM en 60% | Herramienta Online Gratuita",
    heroTitle: "Optimizador de Tokens JSON - Reduce Costos de API LLM en 60%",
    heroSubtitle: "Herramienta online gratuita para convertir JSON a formato TOON, contar tokens y arreglar JSON malformado. Optimiza tus datos para ChatGPT, Claude, GPT-4 y otros Modelos de Lenguaje Grande.",
    toolsHeader: "🛠️ Herramientas del Optimizador de Tokens JSON",
    toolsSubtitle: "Elige una herramienta abajo para optimizar tu JSON para modelos de IA",
    tokenCounterTitle: "Contador de Tokens",
    tokenCounterDesc: "Cuenta tokens en tu texto antes de enviar a APIs de IA",
    jsonRepairTitle: "Herramienta de Reparación JSON",
    jsonRepairDesc: "Arregla automáticamente JSON roto o malformado",
    toonConverterTitle: "Convertidor TOON",
    toonConverterDesc: "Convierte JSON a formato TOON y ahorra hasta 60% en tokens",
    featuresTitle: "¿Por qué usar el Optimizador de Tokens JSON?",
    howItWorksTitle: "Cómo Optimizar JSON para Modelos de Lenguaje Grande",
    comparisonTitle: "JSON vs TOON: Ahorros Reales de Tokens",
    useCasesTitle: "¿Quién se Beneficia de la Optimización de Tokens JSON?",
    faqTitle: "Preguntas Frecuentes",
    footerTitle: "Optimizador de Tokens JSON"
  }
};

function toggleLanguage() {
  currentLanguage = currentLanguage === 'en' ? 'es' : 'en';
  updateLanguage();
  localStorage.setItem('language', currentLanguage);
}

function updateLanguage() {
  const lang = translations[currentLanguage];

  // Update document title
  document.title = lang.title;

  // Update hero section
  document.querySelector('header h1').textContent = lang.heroTitle;
  document.querySelector('.subtitle').textContent = lang.heroSubtitle;

  // Update tools section
  document.querySelector('.tools-header h2').textContent = lang.toolsHeader;
  document.querySelector('.tools-header p').textContent = lang.toolsSubtitle;

  // Update tool cards
  const toolCards = document.querySelectorAll('.tool-card');
  toolCards[0].querySelector('h3').textContent = lang.tokenCounterTitle;
  toolCards[0].querySelector('p').textContent = lang.tokenCounterDesc;
  toolCards[1].querySelector('h3').textContent = lang.jsonRepairTitle;
  toolCards[1].querySelector('p').textContent = lang.jsonRepairDesc;
  toolCards[2].querySelector('h3').textContent = lang.toonConverterTitle;
  toolCards[2].querySelector('p').textContent = lang.toonConverterDesc;

  // Update section titles
  document.querySelector('#features h2').textContent = lang.featuresTitle;
  document.querySelector('#how-it-works h2').textContent = lang.howItWorksTitle;
  document.querySelector('#comparison h2').textContent = lang.comparisonTitle;
  document.querySelector('#use-cases h2').textContent = lang.useCasesTitle;
  document.querySelector('#faq h2').textContent = lang.faqTitle;

  // Update footer
  document.querySelector('.footer-section h4').textContent = lang.footerTitle;

  // Update language toggle button
  document.getElementById('langToggle').textContent = currentLanguage === 'en' ? 'ES' : 'EN';
}

// Initialize language
document.addEventListener('DOMContentLoaded', function() {
  const savedLang = localStorage.getItem('language') || 'en';
  currentLanguage = savedLang;
  updateLanguage();
  updateFAQTranslations(); // Ensure FAQ is also initialized
});

// FAQ Accordion functionality
function toggleFAQ(button) {
  const faqItem = button.parentElement;
  const isActive = faqItem.classList.contains('active');

  // Close all FAQ items
  document.querySelectorAll('.faq-item').forEach(item => {
    item.classList.remove('active');
  });

  // Open clicked item if it wasn't active
  if (!isActive) {
    faqItem.classList.add('active');
  }
}

// Update FAQ translations
function updateFAQTranslations() {
  const faqQuestions = document.querySelectorAll('.faq-question span:first-child');
  const faqAnswers = document.querySelectorAll('.faq-answer p');

  const faqTranslations = {
    en: {
      questions: [
        "What is TOON format and why should I use it?",
        "Does it work with GPT-4, Claude, and other AI models?",
        "Is my data safe and private?",
        "Can it really fix broken JSON automatically?",
        "How accurate is the token counter?",
        "Is there a limit on JSON size?"
      ],
      answers: [
        "TOON (Token-Oriented Object Notation) is a compact data format specifically designed for Large Language Models. It conveys the same information as JSON but uses 30-60% fewer tokens by removing redundant punctuation, using indentation for structure, and tabularizing repeated data. This directly reduces your AI API costs.",
        "Yes! TOON format works with all major LLMs including OpenAI's GPT-4, GPT-3.5, Anthropic's Claude, Google's Gemini, and any other model that processes text input. The token savings apply universally across different tokenizers.",
        "Absolutely. We don't store, log, or transmit your data to any third parties. The tool can work entirely client-side in your browser, or if using our API, data is processed in memory and immediately discarded.",
        "Yes, our intelligent JSON parser can fix common errors like missing closing brackets, trailing commas, unquoted keys, and mismatched quotes. While it can't fix every malformed JSON, it handles the most frequent mistakes developers encounter.",
        "Our token counter uses approximation algorithms similar to GPT tokenizers. While exact counts may vary slightly between models (GPT-4 vs Claude vs Gemini), our counter provides accurate estimates within 5% margin for planning and cost estimation.",
        "For optimal performance, we recommend files under 10MB. Larger files may take longer to process but will still work. There's no hard limit on our free service."
      ]
    },
    es: {
      questions: [
        "¿Qué es el formato TOON y por qué debería usarlo?",
        "¿Funciona con GPT-4, Claude y otros modelos de IA?",
        "¿Mis datos son seguros y privados?",
        "¿Puede arreglar JSON roto automáticamente?",
        "¿Qué tan preciso es el contador de tokens?",
        "¿Hay límite en el tamaño del JSON?"
      ],
      answers: [
        "TOON (Token-Oriented Object Notation) es un formato de datos compacto diseñado específicamente para Modelos de Lenguaje Grande. Transmite la misma información que JSON pero usa 30-60% menos tokens eliminando puntuación redundante, usando indentación para la estructura y tabularizando datos repetidos. Esto reduce directamente tus costos de API de IA.",
        "¡Sí! El formato TOON funciona con todos los LLM principales incluyendo GPT-4 y GPT-3.5 de OpenAI, Claude de Anthropic, Gemini de Google y cualquier otro modelo que procese entrada de texto. Los ahorros de tokens se aplican universalmente en diferentes tokenizadores.",
        "Absolutamente. No almacenamos, registramos ni transmitimos tus datos a terceros. La herramienta puede funcionar completamente del lado del cliente en tu navegador, o si usas nuestra API, los datos se procesan en memoria y se descartan inmediatamente.",
        "Sí, nuestro analizador JSON inteligente puede corregir errores comunes como corchetes de cierre faltantes, comas finales, claves sin comillas y comillas no coincidentes. Aunque no puede arreglar todo JSON malformado, maneja los errores más frecuentes que encuentran los desarrolladores.",
        "Nuestro contador de tokens usa algoritmos de aproximación similares a los tokenizadores GPT. Aunque los conteos exactos pueden variar ligeramente entre modelos (GPT-4 vs Claude vs Gemini), nuestro contador proporciona estimaciones precisas dentro de un margen del 5% para planificación y estimación de costos.",
        "Para un rendimiento óptimo, recomendamos archivos menores a 10MB. Los archivos más grandes pueden tardar más en procesarse pero aún funcionarán. No hay límite estricto en nuestro servicio gratuito."
      ]
    }
  };

  const lang = faqTranslations[currentLanguage];

  faqQuestions.forEach((question, index) => {
    question.textContent = lang.questions[index];
  });

  faqAnswers.forEach((answer, index) => {
    answer.textContent = lang.answers[index];
  });
}

// Update the updateLanguage function to include FAQ
function updateLanguage() {
  const lang = translations[currentLanguage];

  // Update document title
  document.title = lang.title;

  // Update hero section
  document.querySelector('header h1').textContent = lang.heroTitle;
  document.querySelector('.subtitle').textContent = lang.heroSubtitle;

  // Update tools section
  document.querySelector('.tools-header h2').textContent = lang.toolsHeader;
  document.querySelector('.tools-header p').textContent = lang.toolsSubtitle;

  // Update tool cards
  const toolCards = document.querySelectorAll('.tool-card');
  if (toolCards.length >= 3) {
    toolCards[0].querySelector('h3').textContent = lang.tokenCounterTitle;
    toolCards[0].querySelector('p').textContent = lang.tokenCounterDesc;
    toolCards[1].querySelector('h3').textContent = lang.jsonRepairTitle;
    toolCards[1].querySelector('p').textContent = lang.jsonRepairDesc;
    toolCards[2].querySelector('h3').textContent = lang.toonConverterTitle;
    toolCards[2].querySelector('p').textContent = lang.toonConverterDesc;
  }

  // Update section titles
  const featuresTitle = document.querySelector('#features h2');
  const howItWorksTitle = document.querySelector('#how-it-works h2');
  const comparisonTitle = document.querySelector('#comparison h2');
  const useCasesTitle = document.querySelector('#use-cases h2');
  const faqTitle = document.querySelector('#faq h2');

  if (featuresTitle) featuresTitle.textContent = lang.featuresTitle;
  if (howItWorksTitle) howItWorksTitle.textContent = lang.howItWorksTitle;
  if (comparisonTitle) comparisonTitle.textContent = lang.comparisonTitle;
  if (useCasesTitle) useCasesTitle.textContent = lang.useCasesTitle;
  if (faqTitle) faqTitle.textContent = lang.faqTitle;

  // Update footer
  const footerTitle = document.querySelector('.footer-section h4');
  if (footerTitle) footerTitle.textContent = lang.footerTitle;

  // Update FAQ
  updateFAQTranslations();

  // Update language toggle button
  const langToggle = document.getElementById('langToggle');
  if (langToggle) langToggle.textContent = currentLanguage === 'en' ? 'ES' : 'EN';
}

// Initialize copy buttons with accessibility attributes
copyToonBtn.disabled = true;
copyToonBtn.setAttribute('aria-label', 'Copy TOON result to clipboard');

copyFixBtn.disabled = true;
copyFixBtn.setAttribute('aria-label', 'Copy fixed JSON to clipboard');