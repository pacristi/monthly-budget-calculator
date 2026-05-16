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

const renderConfigWidget = (cfg, mesSeleccionado) => {
    if (!cfg) return;
    document.getElementById('config-pct').textContent = `${(cfg.porcentajeParaGastos * 100).toFixed(0)}%`;
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

// Fetch budget data and render summary/gastos
const loadBudget = async () => {
    try {
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

        document.getElementById('val-sueldo').textContent = formatCurrency(data.sueldo);
        document.getElementById('val-presupuesto').textContent = formatCurrency(data.presupuesto_total);
        document.getElementById('val-carga').textContent = formatCurrency(data.carga_actual);
        document.getElementById('val-disponible').textContent = formatCurrency(data.disponible);

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
                    <td>${g.descripcion}</td>
                    <td>${formatCurrency(g.carga)}</td>
                    <td>${g.cuotas > 1 ? g.cuotas : '1'}</td>
                `;
                tbody.appendChild(tr);
            });
        }
    } catch (e) {
        console.error('Error loading budget:', e);
        document.getElementById('gastos-tbody').innerHTML = '<tr><td colspan="4" class="text-center" style="color:red">Error cargando datos.</td></tr>';
    }
};

// Fetch raw movements and render them
const loadMovements = async () => {
    try {
        const response = await fetch('/api/movements');
        const data = await response.json();

        const tbody = document.getElementById('movimientos-tbody');
        tbody.innerHTML = '';
        if (!data || data.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center">No hay movimientos.</td></tr>';
        } else {
            const movimientosOrdenados = [...data].sort((a, b) => b.fecha.localeCompare(a.fecha));
            movimientosOrdenados.forEach(m => {
                const tr = document.createElement('tr');
                let badge = '';
                if (m.miParte !== undefined && m.miParte !== null && m.miParte !== m.monto) {
                    badge = `<span class="badge">mi parte: ${formatCurrency(Math.abs(m.miParte), m.isUsd)}</span>`;
                }
                tr.innerHTML = `
                    <td>${formatFecha(m.fecha)}</td>
                    <td>${m.descripcion} ${badge}</td>
                    <td>${formatCurrency(m.monto, m.isUsd)}</td>
                    <td>
                        <button class="btn btn-primary btn-sm" onclick="openDivideModal('${m.fecha}', '${m.descripcion.replace(/'/g, "\\'")}', ${m.monto}, ${m.isUsd || false}, ${m.miParte !== undefined && m.miParte !== null ? m.miParte : 'null'})">
                            Editar mi parte
                        </button>
                    </td>
                `;
                tbody.appendChild(tr);
            });
        }
    } catch (e) {
        console.error('Error loading movements:', e);
        document.getElementById('movimientos-tbody').innerHTML = '<tr><td colspan="4" class="text-center" style="color:red">Error cargando movimientos.</td></tr>';
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

const loadConfigs = async () => {
    try {
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
                <td>${(c.porcentajeParaGastos * 100).toFixed(1)}%</td>
                <td>${c.diaDeCorteCredito}</td>
                <td>${c.tasaCambioUSD}</td>
                <td class="actions">
                    <button class="btn btn-sm btn-primary" data-action="edit" data-mes="${c.mesDesde}">Editar</button>
                    <button class="btn btn-sm btn-danger" data-action="delete" data-mes="${c.mesDesde}">Eliminar</button>
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

const openConfigModal = (existente = null) => {
    configError.textContent = '';
    const mesInput = document.getElementById('cfg-mes');
    if (existente) {
        document.getElementById('config-modal-title').textContent = `Editar config (${existente.mesDesde})`;
        mesInput.value = existente.mesDesde;
        mesInput.readOnly = true;
        document.getElementById('cfg-pct').value = (existente.porcentajeParaGastos * 100).toFixed(1);
        document.getElementById('cfg-corte').value = existente.diaDeCorteCredito;
        document.getElementById('cfg-usd').value = existente.tasaCambioUSD;
    } else {
        document.getElementById('config-modal-title').textContent = 'Nueva config mensual';
        const now = new Date();
        const yyyymm = `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, '0')}`;
        mesInput.value = yyyymm;
        mesInput.readOnly = false;
        document.getElementById('cfg-pct').value = '';
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
    const payload = {
        porcentajeParaGastos: parseFloat(document.getElementById('cfg-pct').value) / 100,
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

// ======== Navegación de tabs ========

const switchView = (view) => {
    document.querySelectorAll('.nav-tab').forEach(t => {
        t.classList.toggle('active', t.dataset.view === view);
    });
    document.getElementById('view-dashboard').classList.toggle('hidden', view !== 'dashboard');
    document.getElementById('view-configs').classList.toggle('hidden', view !== 'configs');
    document.getElementById('period-selector').classList.toggle('hidden', view !== 'dashboard');

    if (view === 'configs') loadConfigs();
};

document.querySelectorAll('.nav-tab').forEach(t => {
    t.onclick = () => switchView(t.dataset.view);
});

// Initialization
document.addEventListener('DOMContentLoaded', () => {
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

    loadBudget();
    loadProjections();
    loadMovements();
});
