const formatCurrency = (value, isUSD = false) => {
    if (isUSD) {
        return new Intl.NumberFormat('en-US', { style: 'currency', currency: 'USD' }).format(value);
    }
    return new Intl.NumberFormat('es-CL', { style: 'currency', currency: 'CLP' }).format(value);
};

// Fetch budget data and render summary/gastos
const loadBudget = async () => {
    try {
        const response = await fetch('/api/budget');
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
                let badge = m.divididoEn && m.divididoEn > 1 ? `<span class="badge">1/${m.divididoEn}</span>` : '';
                tr.innerHTML = `
                    <td>${m.fecha}</td>
                    <td>${m.descripcion} ${badge}</td>
                    <td>${formatCurrency(m.monto, m.isUsd)}</td>
                    <td>
                        <button class="btn btn-primary btn-sm" onclick="openDivideModal('${m.fecha}', '${m.descripcion.replace(/'/g, "\\'")}', ${m.monto}, ${m.isUsd || false})">
                            Dividir
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

window.openDivideModal = (fecha, descripcion, monto, isUsd = false) => {
    document.getElementById('modal-desc').textContent = descripcion;
    document.getElementById('modal-monto').textContent = `Monto Original: ${formatCurrency(monto, isUsd)}`;
    document.getElementById('input-fecha').value = fecha;
    document.getElementById('input-monto').value = monto;
    document.getElementById('input-personas').value = '1';
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
    const divididoEn = parseInt(document.getElementById('input-personas').value);

    try {
        await fetch('/api/divisions', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({
                fecha: fecha,
                montoOriginal: montoOriginal,
                divididoEn: divididoEn
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

// Initialization
document.addEventListener('DOMContentLoaded', () => {
    loadBudget();
    loadMovements();
});
