const formatCurrency = (value, isUSD = false) => {
    if (isUSD) {
        return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value);
    }
    return new Intl.NumberFormat('es-CL', { style: 'currency', currency: 'CLP' }).format(value);
};

// Fetch budget data and render summary/gastos
const loadBudget = async () => {
    try {
        let url = '/api/budget';
        const selector = document.getElementById('period-selector');
        if (selector && selector.value) {
            const [year, month] = selector.value.split('-');
            url += `?year=${year}&month=${month}`;
        }
        
        const response = await fetch(url);
        const data = await response.json();

        document.getElementById('val-sueldo').textContent = formatCurrency(data.sueldo);
        document.getElementById('val-presupuesto').textContent = formatCurrency(data.presupuesto_total);
        document.getElementById('val-carga').textContent = formatCurrency(data.carga_actual);
        document.getElementById('val-disponible').textContent = formatCurrency(data.disponible);

        const tbody = document.getElementById('gastos-tbody');
        tbody.innerHTML = '';
        if (!data.gastos || data.gastos.length === 0) {
            tbody.innerHTML = '<tr><td colspan="4" class="text-center">No hay gastos para este mes.</td></tr>';
        } else {
            data.gastos.forEach(g => {
                const tr = document.createElement('tr');
                tr.innerHTML = `
                    <td>${g.fecha}</td>
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
            data.forEach(m => {
                const tr = document.createElement('tr');
                let badge = '';
                if (m.miParte !== undefined && m.miParte !== null && m.miParte !== m.monto) {
                    badge = `<span class="badge">mi parte: ${formatCurrency(Math.abs(m.miParte), m.isUsd)}</span>`;
                }
                tr.innerHTML = `
                    <td>${m.fecha}</td>
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

// Modal Logic
const modal = document.getElementById('divide-modal');
const closeBtn = document.getElementById('close-modal');
const form = document.getElementById('divide-form');

window.openDivideModal = (fecha, descripcion, monto, isUsd = false, miParteActual = null) => {
    document.getElementById('modal-desc').textContent = descripcion;
    document.getElementById('modal-monto').textContent = `Monto Original: ${formatCurrency(monto, isUsd)}`;
    document.getElementById('input-fecha').value = fecha;
    document.getElementById('input-monto').value = monto;
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
window.onclick = (e) => {
    if (e.target == modal) {
        modal.classList.remove('active');
    }
};

form.onsubmit = async (e) => {
    e.preventDefault();
    const fecha = document.getElementById('input-fecha').value;
    const montoOriginal = parseFloat(document.getElementById('input-monto').value);
    // El usuario ingresa positivo; persistimos con el mismo signo del original.
    const miParteAbs = Math.abs(parseFloat(document.getElementById('input-mi-parte').value));
    const miParte = montoOriginal < 0 ? -miParteAbs : miParteAbs;

    try {
        await fetch('/api/divisions', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                fecha: fecha,
                montoOriginal: montoOriginal,
                miParte: miParte
            })
        });

        modal.classList.remove('active');
        // Reload everything
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

