const IGNORAR = 'ignorado';
let categoriasCache = [];

const formatFecha = (iso) => {
    const m = iso.match(/^(\d{4})-(\d{2})-(\d{2})/);
    return m ? `${m[3]}-${m[2]}-${m[1]}` : iso;
};

const formatCurrency = (value, isUSD = false) => {
    if (isUSD) {
        return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value);
    }
    return new Intl.NumberFormat('es-CL', { style: 'currency', currency: 'CLP' }).format(value);
};

const escapeHtml = (value) => String(value ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#39;');

const jsArg = (value) => JSON.stringify(String(value ?? '')).replace(/</g, '\\u003c');

// ======== Modo "Ocultar valores sensibles" ========
// Enmascara los montos en $ derivados del sueldo (tarjeta de sueldo + barras +
// sin-asignar) para poder mostrarle la app a terceros sin revelar el ingreso.
// Es "privacidad de hombro": el dato sigue en el estado, solo se oculta en pantalla.
// Persiste en localStorage para que un reload no exponga los valores sin querer.
let valoresOcultos = localStorage.getItem('valoresOcultos') === '1';

const PUNTOS_OCULTO = '<span class="valor-oculto">••••••</span>';

// fmtSensible devuelve el monto formateado, o puntos si el modo ocultar está activo.
// Devuelve HTML, así que usar con innerHTML (no textContent).
const fmtSensible = (value, isUSD = false) =>
    valoresOcultos ? PUNTOS_OCULTO : formatCurrency(value, isUSD);

const toggleValoresOcultos = () => {
    valoresOcultos = !valoresOcultos;
    localStorage.setItem('valoresOcultos', valoresOcultos ? '1' : '0');
    sincronizarBotonOcultar();
    loadBudget(); // re-render con/sin máscara (re-fetch local, barato)
};

const sincronizarBotonOcultar = () => {
    const btn = document.getElementById('btn-ocultar');
    if (btn) btn.textContent = valoresOcultos ? '👁 Mostrar' : '🙈 Ocultar';
};

const ensureCategorias = async () => {
    if (categoriasCache.length > 0) return categoriasCache;
    try {
        const res = await fetch('/api/categorias');
        categoriasCache = await res.json();
    } catch (e) {
        console.error('Error cargando categorías:', e);
        categoriasCache = [];
    }
    return categoriasCache;
};

const nombreCategoria = (id) => {
    const c = categoriasCache.find(c => c.id === id);
    return c ? c.nombre : id;
};

const renderConfigWidget = (cfg, mesSeleccionado) => {
    if (!cfg) return;
    document.getElementById('config-corte').textContent = cfg.diaDeCorteCredito;
    document.getElementById('config-usd').textContent = cfg.tasaCambioUSD;

    const fuente = document.getElementById('config-source');
    if (cfg.heredadaDe === mesSeleccionado) {
        fuente.textContent = 'Config propia';
        fuente.classList.remove('inherited');
    } else {
        fuente.textContent = `Heredada de ${cfg.heredadaDe}`;
        fuente.classList.add('inherited');
    }
};

// renderBars dibuja una barra por categoría. Meta: vacío=malo, lleno=bueno (verde).
// Límite: vacío=bueno, lleno=malo (rojo al acercarse al tope).
const renderBars = (categorias, sinAsignar) => {
    const cont = document.getElementById('barras-container');
    cont.innerHTML = '';
    if (!categorias || categorias.length === 0) {
        cont.innerHTML = '<p class="text-center" style="color:var(--text-secondary)">No hay categorías. Definilas en Configuración.</p>';
        return;
    }

    categorias.forEach(c => {
        const presupuesto = c.presupuesto || 0;
        const acumulado = c.acumulado || 0;
        const ratio = presupuesto > 0 ? acumulado / presupuesto : 0;
        const pctReal = Math.round(ratio * 100);
        const ancho = Math.min(100, Math.max(0, ratio * 100));
        const restante = presupuesto - acumulado;
        const esMeta = c.tipo === 'meta';
        const over = ratio > 1.0001;

        let pie;
        if (esMeta) {
            pie = restante > 0
                ? `te faltan ${fmtSensible(restante)}`
                : `meta cumplida (+${fmtSensible(-restante)})`;
        } else {
            pie = restante >= 0
                ? `te quedan ${fmtSensible(restante)}`
                : `te pasaste ${fmtSensible(-restante)}`;
        }

        const clases = ['barra-item', esMeta ? 'barra-meta' : 'barra-limite'];
        if (over) clases.push(esMeta ? 'barra-meta-over' : 'barra-limite-over');
        if (presupuesto <= 0) clases.push('barra-sin-pct');

        const div = document.createElement('div');
        div.className = clases.join(' ');
        div.innerHTML = `
            <div class="barra-head">
                <span class="barra-nombre">${c.nombre} <span class="barra-tag">${esMeta ? 'meta' : 'límite'} · ${Math.round((c.porcentaje || 0) * 100)}%</span></span>
                <span class="barra-real">${presupuesto > 0 ? pctReal + '%' : 'sin %'}</span>
            </div>
            <div class="barra-track"><div class="barra-fill" style="width:${ancho}%"></div></div>
            <div class="barra-foot">${fmtSensible(acumulado)} de ${fmtSensible(presupuesto)} · <strong>${pie}</strong></div>
        `;
        cont.appendChild(div);
    });

    const sa = document.getElementById('sin-asignar');
    if (Math.abs(sinAsignar) < 1) {
        sa.innerHTML = '';
    } else if (sinAsignar > 0) {
        sa.innerHTML = `Sin asignar este mes: ${fmtSensible(sinAsignar)}`;
        sa.className = 'sin-asignar';
    } else {
        sa.innerHTML = `Asignaste más del 100% del sueldo: ${fmtSensible(-sinAsignar)} de más`;
        sa.className = 'sin-asignar sin-asignar-over';
    }
};

// Fetch budget data and render bars + gastos
const loadBudget = async () => {
    try {
        await ensureCategorias();
        let url = '/api/budget';
        let mesSeleccionado = null;
        const selector = document.getElementById('period-selector');
        if (selector && selector.value) {
            const [year, month] = selector.value.split('-');
            url += `?year=${year}&month=${month}`;
            mesSeleccionado = selector.value;
        }

        const response = await fetch(url);
        const data = await response.json();

        document.getElementById('val-sueldo').innerHTML = fmtSensible(data.sueldo);
        renderBars(data.categorias, data.sinAsignar);
        renderConfigWidget(data.config, mesSeleccionado);

        const tbody = document.getElementById('gastos-tbody');
        tbody.innerHTML = '';
        if (!data.gastos || data.gastos.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center">No hay gastos para este mes.</td></tr>';
        } else {
            const gastosOrdenados = [...data.gastos].sort((a, b) => b.fecha.localeCompare(a.fecha));
            gastosOrdenados.forEach(g => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td>${formatFecha(g.fecha)}</td>
                    <td>${escapeHtml(g.descripcion)} <span class="cat-chip">${nombreCategoria(g.categoriaId)}</span></td>
                    <td>${formatCurrency(g.carga)}</td>
                    <td>${g.cuotas > 1 ? g.cuotas : '1'}</td>
                `;
                tbody.appendChild(tr);
            });
        }
    } catch (e) {
        console.error('Error loading budget:', e);
        document.getElementById('barras-container').innerHTML = '<p class="text-center" style="color:var(--danger)">Error cargando datos.</p>';
    }
};

// Construye un <select> de categorías (+ ignorar) con el valor actual marcado.
const buildCategoriaSelect = (m) => {
    const opciones = categoriasCache.map(c =>
        `<option value="${c.id}" ${c.id === m.categoriaId ? 'selected' : ''}>${c.nombre}</option>`
    );
    opciones.push(`<option value="${IGNORAR}" ${m.categoriaId === IGNORAR ? 'selected' : ''}>— ignorar —</option>`);
    const descripcionOriginal = m.descripcionOriginal || m.descripcion;
    return `<select class="cat-select" onchange="setMovimientoCategoria(${jsArg(m.fecha)}, ${m.monto}, ${jsArg(descripcionOriginal)}, this.value)">${opciones.join('')}</select>`;
};

window.setMovimientoCategoria = async (fecha, monto, descripcion, categoria) => {
    try {
        const res = await fetch('/api/movimientos/categoria', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ fecha, montoOriginal: monto, descripcion, categoria })
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        loadBudget();
    } catch (e) {
        console.error('Error asignando categoría:', e);
        alert('Error al asignar categoría (¿configuraste un archivo de divisiones?)');
    }
};

window.setMovimientoNombre = async (fecha, monto, descripcion, nombre) => {
    try {
        const res = await fetch('/api/movimientos/nombre', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ fecha, montoOriginal: monto, descripcion, nombre: nombre.trim() })
        });
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        loadBudget();
        loadMovements();
    } catch (e) {
        console.error('Error renombrando movimiento:', e);
        alert('Error al cambiar el nombre del movimiento');
    }
};

const buildNombreInput = (m) => {
    const descripcionOriginal = m.descripcionOriginal || m.descripcion;
    const title = descripcionOriginal !== m.descripcion ? ` title="Original: ${escapeHtml(descripcionOriginal)}"` : '';
    return `
        <div class="movement-name"${title}>
            <input class="movement-name-input" type="text" value="${escapeHtml(m.descripcion)}" aria-label="Nombre del movimiento" onkeydown="if(event.key === 'Enter') this.blur()" onchange="setMovimientoNombre(${jsArg(m.fecha)}, ${m.monto}, ${jsArg(descripcionOriginal)}, this.value)">
            ${descripcionOriginal !== m.descripcion ? `<span class="movement-original">Original: ${escapeHtml(descripcionOriginal)}</span>` : ''}
        </div>
    `;
};

// Fetch raw movements and render them
const loadMovements = async () => {
    try {
        await ensureCategorias();
        const response = await fetch('/api/movements');
        const data = await response.json();

        const tbody = document.getElementById('movimientos-tbody');
        tbody.innerHTML = '';
        if (!data || data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center">No hay movimientos.</td></tr>';
        } else {
            const movimientosOrdenados = [...data].sort((a, b) => b.fecha.localeCompare(a.fecha));
            movimientosOrdenados.forEach(m => {
                const tr = document.createElement('tr');
                let badge = '';
                if (m.miParte !== undefined && m.miParte !== null && m.miParte !== m.monto) {
                    badge = `<span class="badge">mi parte: ${formatCurrency(Math.abs(m.miParte), m.isUsd)}</span>`;
                }
                const descripcionOriginal = m.descripcionOriginal || m.descripcion;
                const miParteArg = m.miParte !== undefined && m.miParte !== null ? m.miParte : 'null';
                tr.innerHTML = `
                    <td>${formatFecha(m.fecha)}</td>
                    <td>${buildNombreInput(m)} ${badge}</td>
                    <td>${formatCurrency(m.monto, m.isUsd)}</td>
                    <td>${buildCategoriaSelect(m)}</td>
                    <td>
                        <button class="btn btn-primary btn-sm" onclick="openDivideModal(${jsArg(m.fecha)}, ${jsArg(descripcionOriginal)}, ${m.monto}, ${m.isUsd || false}, ${miParteArg})">
                            Mi parte
                        </button>
                        <button class="btn btn-secondary btn-sm" onclick="ignorarGasto(${jsArg(m.fecha)}, ${jsArg(descripcionOriginal)}, ${m.monto})" title="Marca este gasto como no contable (mi parte = 0)">
                            No contar
                        </button>
                    </td>
                `;
                tbody.appendChild(tr);
            });
        }
    } catch (e) {
        console.error('Error loading movements:', e);
        document.getElementById('movimientos-tbody').innerHTML = '<tr><td colspan="5" class="text-center" style="color:red">Error cargando movimientos.</td></tr>';
    }
};

// Modal Logic Dividir
const modal = document.getElementById('divide-modal');
const closeBtn = document.getElementById('close-modal');
const form = document.getElementById('divide-form');

window.openDivideModal = (fecha, descripcion, monto, isUsd = false, miParteActual = null) => {
    document.getElementById('modal-desc').textContent = descripcion;
    document.getElementById('modal-monto').textContent = `Monto Original: ${formatCurrency(monto, isUsd)}`;
    document.getElementById('input-fecha').value = fecha;
    document.getElementById('input-monto').value = monto;
    document.getElementById('input-descripcion').value = descripcion;
    document.getElementById('input-is-usd').value = isUsd ? '1' : '0';

    const absMonto = Math.abs(monto);
    const miParteInput = document.getElementById('input-mi-parte');
    miParteInput.value = miParteActual !== null ? Math.abs(miParteActual) : (absMonto / 2).toFixed(2);

    const hint = document.getElementById('hint-presets');
    const div2 = (absMonto / 2).toFixed(2);
    const div3 = (absMonto / 3).toFixed(2);
    const div4 = (absMonto / 4).toFixed(2);
    hint.innerHTML = `Presets: <a href="#" data-v="${div2}">÷2 (${div2})</a> · <a href="#" data-v="${div3}">÷3 (${div3})</a> · <a href="#" data-v="${div4}">÷4 (${div4})</a>`;
    hint.querySelectorAll('a').forEach(a => {
        a.onclick = (ev) => { ev.preventDefault(); miParteInput.value = a.dataset.v; };
    });

    modal.classList.add('active');
};

// ignorarGasto crea un override con miParte=0 para que el gasto no impacte
// el presupuesto. Atajo para gastos compartidos donde no me toca nada.
window.ignorarGasto = async (fecha, descripcion, monto) => {
    if (!confirm(`¿Marcar "${descripcion}" (${formatCurrency(monto, false)}) como no contable?`)) {
        return;
    }
    try {
        const res = await fetch('/api/divisions', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ fecha, montoOriginal: monto, descripcion, miParte: 0 })
        });
        if (!res.ok) {
            throw new Error(`HTTP ${res.status}`);
        }
        loadBudget();
        loadMovements();
    } catch (e) {
        console.error('Error ignorando gasto:', e);
        alert('Error al marcar como no contable');
    }
};

closeBtn.onclick = () => modal.classList.remove('active');
window.addEventListener('click', (e) => {
    if (e.target === modal) modal.classList.remove('active');
    if (e.target === document.getElementById('config-modal')) document.getElementById('config-modal').classList.remove('active');
});

form.onsubmit = async (e) => {
    e.preventDefault();
    const fecha = document.getElementById('input-fecha').value;
    const montoOriginal = parseFloat(document.getElementById('input-monto').value);
    const descripcion = document.getElementById('input-descripcion').value;
    const miParteAbs = Math.abs(parseFloat(document.getElementById('input-mi-parte').value));
    const miParte = montoOriginal < 0 ? -miParteAbs : miParteAbs;

    try {
        await fetch('/api/divisions', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ fecha, montoOriginal, descripcion, miParte })
        });

        modal.classList.remove('active');
        loadBudget();
        loadMovements();
    } catch (e) {
        console.error('Error saving division:', e);
        alert('Error al guardar división');
    }
};

// Fetch projections
const loadProjections = async () => {
    try {
        const response = await fetch('/api/projections');
        const data = await response.json();

        const tbody = document.getElementById('proyecciones-tbody');
        tbody.innerHTML = '';
        if (!data || data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="3" class="text-center">No hay proyecciones disponibles.</td></tr>';
        } else {
            data.forEach(p => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td>${p.anio}</td>
                    <td>${p.mes}</td>
                    <td>${formatCurrency(p.totalComprometido)}</td>
                `;
                tbody.appendChild(tr);
            });
        }
    } catch (e) {
        console.error('Error loading projections:', e);
        document.getElementById('proyecciones-tbody').innerHTML = '<tr><td colspan="3" class="text-center" style="color:red">Error cargando proyecciones.</td></tr>';
    }
};

// ======== Configs por mes ========

const configsTbody = () => document.getElementById('configs-tbody');
const configModal = document.getElementById('config-modal');
const configForm = document.getElementById('config-form');
const configError = document.getElementById('cfg-error');

const resumenPorcentajes = (c) => {
    if (c.porcentajes && Object.keys(c.porcentajes).length > 0) {
        return Object.entries(c.porcentajes)
            .map(([id, p]) => `${nombreCategoria(id)} ${Math.round(p * 100)}%`)
            .join(' · ');
    }
    // legacy
    return `gasto ${((c.porcentajeParaGastos || 0) * 100).toFixed(0)}%`;
};

const loadConfigs = async () => {
    try {
        await ensureCategorias();
        const response = await fetch('/api/configs');
        const data = await response.json();
        const tbody = configsTbody();
        tbody.innerHTML = '';
        if (!data || data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="5" class="text-center">Sin configs declaradas.</td></tr>';
            return;
        }
        data.forEach(c => {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td>${c.mesDesde}</td>
                <td>${resumenPorcentajes(c)}</td>
                <td>${c.diaDeCorteCredito}</td>
                <td>${c.tasaCambioUSD}</td>
                <td class="actions">
                    <button class="btn btn-sm btn-primary" data-action="edit">Editar</button>
                    <button class="btn btn-sm btn-danger" data-action="delete">Eliminar</button>
                </td>
            `;
            tr.querySelector('[data-action="edit"]').onclick = () => openConfigModal(c);
            tr.querySelector('[data-action="delete"]').onclick = () => deleteConfig(c.mesDesde);
            tbody.appendChild(tr);
        });
    } catch (e) {
        console.error('Error loading configs:', e);
        configsTbody().innerHTML = '<tr><td colspan="5" class="text-center" style="color:red">Error cargando configs.</td></tr>';
    }
};

// Pinta un input de % por cada categoría existente, prellenando desde la config.
const renderPorcentajeInputs = (porcentajes = {}) => {
    const cont = document.getElementById('cfg-porcentajes');
    cont.innerHTML = '';
    categoriasCache.forEach(c => {
        const val = porcentajes[c.id] !== undefined ? (porcentajes[c.id] * 100) : '';
        const row = document.createElement('div');
        row.className = 'cfg-pct-row';
        row.innerHTML = `
            <label>${c.nombre} <span class="barra-tag">${c.tipo}</span></label>
            <input type="number" step="0.1" min="0" max="100" data-cat="${c.id}" value="${val}">
        `;
        cont.appendChild(row);
    });
    actualizarSumaPorcentajes();
    cont.querySelectorAll('input').forEach(i => i.addEventListener('input', actualizarSumaPorcentajes));
};

const actualizarSumaPorcentajes = () => {
    const inputs = document.querySelectorAll('#cfg-porcentajes input');
    let suma = 0;
    inputs.forEach(i => { suma += parseFloat(i.value) || 0; });
    const el = document.getElementById('cfg-suma');
    el.textContent = `Suma: ${suma.toFixed(0)}%${suma === 100 ? ' ✓' : (suma > 100 ? ' (más de 100%)' : ' (queda sin asignar)')}`;
    el.style.color = suma === 100 ? 'var(--success)' : 'var(--text-secondary)';
};

const openConfigModal = async (existente = null) => {
    configError.textContent = '';
    await ensureCategorias();
    const mesInput = document.getElementById('cfg-mes');
    if (existente) {
        document.getElementById('config-modal-title').textContent = `Editar config (${existente.mesDesde})`;
        mesInput.value = existente.mesDesde;
        mesInput.readOnly = true;
        const porc = existente.porcentajes && Object.keys(existente.porcentajes).length > 0
            ? existente.porcentajes
            : (existente.porcentajeParaGastos ? { gasto: existente.porcentajeParaGastos } : {});
        renderPorcentajeInputs(porc);
        document.getElementById('cfg-corte').value = existente.diaDeCorteCredito;
        document.getElementById('cfg-usd').value = existente.tasaCambioUSD;
    } else {
        document.getElementById('config-modal-title').textContent = 'Nueva config mensual';
        const now = new Date();
        const yyyymm = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
        mesInput.value = yyyymm;
        mesInput.readOnly = false;
        renderPorcentajeInputs({});
        document.getElementById('cfg-corte').value = '';
        document.getElementById('cfg-usd').value = '';
    }
    configModal.classList.add('active');
};

document.getElementById('close-config-modal').onclick = () => configModal.classList.remove('active');

configForm.onsubmit = async (e) => {
    e.preventDefault();
    configError.textContent = '';
    const mesDesde = document.getElementById('cfg-mes').value;

    const porcentajes = {};
    document.querySelectorAll('#cfg-porcentajes input').forEach(i => {
        const v = parseFloat(i.value);
        if (!isNaN(v) && v !== 0) porcentajes[i.dataset.cat] = v / 100;
    });

    const payload = {
        porcentajes,
        diaDeCorteCredito: parseInt(document.getElementById('cfg-corte').value, 10),
        tasaCambioUSD: parseFloat(document.getElementById('cfg-usd').value),
    };
    try {
        const res = await fetch(`/api/configs/${encodeURIComponent(mesDesde)}`, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(payload),
        });
        if (!res.ok) {
            const txt = await res.text();
            configError.textContent = txt || `Error ${res.status}`;
            return;
        }
        configModal.classList.remove('active');
        await loadConfigs();
        if (!document.getElementById('view-dashboard').classList.contains('hidden')) {
            loadBudget();
        }
    } catch (err) {
        console.error('Error guardando config:', err);
        configError.textContent = String(err);
    }
};

const deleteConfig = async (mes) => {
    if (!confirm(`Eliminar config para ${mes}? Los meses que la heredaban pasarán a heredar de una anterior.`)) return;
    const res = await fetch(`/api/configs/${encodeURIComponent(mes)}`, { method: 'DELETE' });
    if (!res.ok) {
        const txt = await res.text();
        alert(txt || `Error ${res.status}`);
        return;
    }
    await loadConfigs();
    if (!document.getElementById('view-dashboard').classList.contains('hidden')) {
        loadBudget();
    }
};

document.getElementById('btn-nueva-config').onclick = () => openConfigModal(null);

// ======== Categorías ========

const slugify = (s) => s.toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '')
    .replace(/[^a-z0-9]+/g, '-').replace(/^-+|-+$/g, '');

const renderCategorias = (cats) => {
    const ul = document.getElementById('lista-categorias');
    ul.innerHTML = '';
    if (!cats || cats.length === 0) {
        ul.innerHTML = '<li class="lista-strings-empty">Sin categorías.</li>';
        return;
    }
    cats.forEach((c, idx) => {
        const li = document.createElement('li');
        li.className = c.tipo === 'meta' ? 'cat-pill cat-meta' : 'cat-pill cat-limite';
        const span = document.createElement('span');
        span.textContent = `${c.nombre} · ${c.tipo}`;
        li.appendChild(span);
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.textContent = '×';
        btn.title = 'Quitar';
        btn.onclick = () => deleteCategoria(idx);
        li.appendChild(btn);
        ul.appendChild(li);
    });
};

const guardarCategorias = async (cats) => {
    const res = await fetch('/api/categorias', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(cats),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    categoriasCache = cats;
};

const loadCategorias = async () => {
    try {
        const res = await fetch('/api/categorias');
        categoriasCache = await res.json();
        renderCategorias(categoriasCache);
        renderReglaDestinoOptions();
    } catch (e) {
        console.error('Error cargando categorías:', e);
        document.getElementById('lista-categorias').innerHTML =
            `<li class="lista-strings-empty" style="color:var(--danger)">Error: ${e.message}</li>`;
    }
};

const addCategoria = async () => {
    const nombre = document.getElementById('input-cat-nombre').value.trim();
    const tipo = document.getElementById('input-cat-tipo').value;
    if (nombre === '') return;
    let id = slugify(nombre);
    if (id === '') { alert('Nombre inválido'); return; }
    if (categoriasCache.some(c => c.id === id)) {
        alert('Ya existe una categoría con ese nombre.');
        return;
    }
    try {
        const nuevas = [...categoriasCache, { id, nombre, tipo }];
        await guardarCategorias(nuevas);
        document.getElementById('input-cat-nombre').value = '';
        renderCategorias(nuevas);
        renderReglaDestinoOptions();
    } catch (e) {
        alert(`Error al agregar categoría: ${e.message}`);
    }
};

const deleteCategoria = async (idx) => {
    if (idx < 0 || idx >= categoriasCache.length) return;
    const cat = categoriasCache[idx];
    if (!confirm(`¿Quitar la categoría "${cat.nombre}"? Sus % en las configs dejarán de aplicarse.`)) return;
    try {
        const nuevas = categoriasCache.filter((_, i) => i !== idx);
        await guardarCategorias(nuevas);
        renderCategorias(nuevas);
        renderReglaDestinoOptions();
    } catch (e) {
        alert(`Error al quitar categoría: ${e.message}`);
    }
};

// ======== Reglas de categorización ========

const renderReglaDestinoOptions = () => {
    const sel = document.getElementById('input-regla-destino');
    if (!sel) return;
    const prev = sel.value;
    sel.innerHTML = categoriasCache.map(c => `<option value="${c.id}">${c.nombre}</option>`).join('')
        + `<option value="${IGNORAR}">— ignorar —</option>`;
    if (prev) sel.value = prev;
};

let reglasCache = [];

const destinoLabel = (destino) => destino === IGNORAR ? 'ignorar' : nombreCategoria(destino);

const renderReglas = (reglas) => {
    const ul = document.getElementById('lista-reglas');
    ul.innerHTML = '';
    if (!reglas || reglas.length === 0) {
        ul.innerHTML = '<li class="lista-strings-empty">Sin reglas. Todo lo no clasificado va a gasto.</li>';
        return;
    }
    reglas.forEach((r, idx) => {
        const li = document.createElement('li');
        li.className = 'regla-item';
        const span = document.createElement('span');
        span.innerHTML = `<code>${r.patron}</code> → <strong>${destinoLabel(r.destino)}</strong>`;
        li.appendChild(span);
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.textContent = '×';
        btn.title = 'Quitar';
        btn.onclick = () => deleteRegla(idx);
        li.appendChild(btn);
        ul.appendChild(li);
    });
};

const guardarReglas = async (reglas) => {
    const res = await fetch('/api/reglas', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(reglas),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    reglasCache = reglas;
};

const loadReglas = async () => {
    try {
        const res = await fetch('/api/reglas');
        reglasCache = await res.json();
        renderReglas(reglasCache);
    } catch (e) {
        console.error('Error cargando reglas:', e);
        document.getElementById('lista-reglas').innerHTML =
            `<li class="lista-strings-empty" style="color:var(--danger)">Error: ${e.message}</li>`;
    }
};

const addRegla = async () => {
    const patron = document.getElementById('input-regla-patron').value.trim();
    const destino = document.getElementById('input-regla-destino').value;
    if (patron === '' || !destino) return;
    try {
        const nuevas = [...reglasCache, { patron, destino }];
        await guardarReglas(nuevas);
        document.getElementById('input-regla-patron').value = '';
        renderReglas(nuevas);
        loadBudget();
        loadMovements();
    } catch (e) {
        alert(`Error al agregar regla: ${e.message}`);
    }
};

const deleteRegla = async (idx) => {
    if (idx < 0 || idx >= reglasCache.length) return;
    if (!confirm(`¿Quitar la regla "${reglasCache[idx].patron}"?`)) return;
    try {
        const nuevas = reglasCache.filter((_, i) => i !== idx);
        await guardarReglas(nuevas);
        renderReglas(nuevas);
        loadBudget();
        loadMovements();
    } catch (e) {
        alert(`Error al quitar regla: ${e.message}`);
    }
};

// ======== Navegación de tabs ========

const switchView = (view) => {
    document.querySelectorAll('.nav-tab').forEach(t => {
        t.classList.toggle('active', t.dataset.view === view);
    });
    document.getElementById('view-dashboard').classList.toggle('hidden', view !== 'dashboard');
    document.getElementById('view-configs').classList.toggle('hidden', view !== 'configs');
    document.getElementById('period-selector').classList.toggle('hidden', view !== 'dashboard');

    if (view === 'configs') {
        loadCategorias();
        loadReglas();
        loadConfigs();
        loadListaStrings('sueldo');
    }
};

// ======== Listas editables (patrones de sueldo) ========

const listaStringsConfig = {
    sueldo: {
        endpoint: '/api/sueldo',
        ulId: 'lista-sueldo',
        inputId: 'input-nuevo-sueldo',
        btnId: 'btn-agregar-sueldo',
        empty: 'Sin patrones cargados. Sin esto el sistema no detecta tu sueldo.',
    },
};

const renderListaStrings = (key, items) => {
    const cfg = listaStringsConfig[key];
    const ul = document.getElementById(cfg.ulId);
    ul.innerHTML = '';
    if (!items || items.length === 0) {
        const li = document.createElement('li');
        li.className = 'lista-strings-empty';
        li.textContent = cfg.empty;
        ul.appendChild(li);
        return;
    }
    items.forEach((s, idx) => {
        const li = document.createElement('li');
        const span = document.createElement('span');
        span.textContent = s;
        li.appendChild(span);
        const btn = document.createElement('button');
        btn.type = 'button';
        btn.textContent = '×';
        btn.title = 'Quitar';
        btn.onclick = () => removerDeListaStrings(key, idx);
        li.appendChild(btn);
        ul.appendChild(li);
    });
};

const loadListaStrings = async (key) => {
    const cfg = listaStringsConfig[key];
    try {
        const res = await fetch(cfg.endpoint);
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        const items = await res.json();
        renderListaStrings(key, items);
    } catch (e) {
        console.error(`Error cargando ${key}:`, e);
        document.getElementById(cfg.ulId).innerHTML =
            `<li class="lista-strings-empty" style="color:var(--danger)">Error cargando: ${e.message}</li>`;
    }
};

const guardarListaStrings = async (key, items) => {
    const cfg = listaStringsConfig[key];
    const res = await fetch(cfg.endpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(items),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
};

const agregarAListaStrings = async (key) => {
    const cfg = listaStringsConfig[key];
    const input = document.getElementById(cfg.inputId);
    const valor = input.value.trim();
    if (valor === '') return;

    try {
        const actuales = await (await fetch(cfg.endpoint)).json();
        if (actuales.includes(valor)) {
            alert('Esa entrada ya existe.');
            return;
        }
        const nuevos = [...actuales, valor];
        await guardarListaStrings(key, nuevos);
        input.value = '';
        renderListaStrings(key, nuevos);
        loadBudget();
        loadMovements();
    } catch (e) {
        console.error(`Error agregando a ${key}:`, e);
        alert(`Error al agregar: ${e.message}`);
    }
};

const removerDeListaStrings = async (key, idx) => {
    const cfg = listaStringsConfig[key];
    try {
        const actuales = await (await fetch(cfg.endpoint)).json();
        if (idx < 0 || idx >= actuales.length) return;
        if (!confirm(`¿Quitar "${actuales[idx]}"?`)) return;
        const nuevos = actuales.filter((_, i) => i !== idx);
        await guardarListaStrings(key, nuevos);
        renderListaStrings(key, nuevos);
        loadBudget();
        loadMovements();
    } catch (e) {
        console.error(`Error removiendo de ${key}:`, e);
        alert(`Error al quitar: ${e.message}`);
    }
};

// Wire-up
document.getElementById('btn-agregar-sueldo').onclick = () => agregarAListaStrings('sueldo');
document.getElementById('input-nuevo-sueldo').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); agregarAListaStrings('sueldo'); }
});

document.getElementById('btn-agregar-categoria').onclick = () => addCategoria();
document.getElementById('input-cat-nombre').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); addCategoria(); }
});

document.getElementById('btn-agregar-regla').onclick = () => addRegla();
document.getElementById('input-regla-patron').addEventListener('keydown', (e) => {
    if (e.key === 'Enter') { e.preventDefault(); addRegla(); }
});

document.querySelectorAll('.nav-tab').forEach(t => {
    t.onclick = () => switchView(t.dataset.view);
});

// Initialization
// refrescar dispara el scraper en el server (POST /api/refresh) y, al terminar,
// recarga las vistas del dashboard. Es síncrono: el botón queda deshabilitado
// mientras corre (puede tardar varios segundos).
const refrescar = async () => {
    const btn = document.getElementById('btn-refresh');
    const textoOriginal = btn ? btn.textContent : '';
    if (btn) {
        btn.disabled = true;
        btn.textContent = '↻ Refrescando...';
    }
    try {
        const res = await fetch('/api/refresh', { method: 'POST' });
        if (!res.ok) {
            throw new Error(await res.text());
        }
        await ensureCategorias();
        loadBudget();
        loadProjections();
        loadMovements();
    } catch (e) {
        alert('No se pudo refrescar: ' + e.message);
    } finally {
        if (btn) {
            btn.disabled = false;
            btn.textContent = textoOriginal;
        }
    }
};

document.addEventListener('DOMContentLoaded', async () => {
    const selector = document.getElementById('period-selector');
    if (selector) {
        const now = new Date();
        const year = now.getFullYear();
        const month = String(now.getMonth() + 1).padStart(2, '0');
        selector.value = `${year}-${month}`;

        selector.addEventListener('change', () => {
            loadBudget();
        });
    }

    const btnRefresh = document.getElementById('btn-refresh');
    if (btnRefresh) {
        btnRefresh.addEventListener('click', refrescar);
    }

    const btnOcultar = document.getElementById('btn-ocultar');
    if (btnOcultar) {
        btnOcultar.addEventListener('click', toggleValoresOcultos);
        sincronizarBotonOcultar();
    }

    await ensureCategorias();
    loadBudget();
    loadProjections();
    loadMovements();
});
