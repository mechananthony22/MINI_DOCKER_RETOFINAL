const runForm = document.getElementById('runForm');
const containersTable = document.querySelector('#containersTable tbody');

// Fetch active containers periodically
async function fetchContainers() {
    try {
        const res = await fetch('/api/containers');
        const containers = await res.json();
        
        containersTable.innerHTML = '';
        
        if (!containers || containers.length === 0) {
            containersTable.innerHTML = `<tr><td colspan="3" style="text-align:center; color:#94a3b8">No active containers</td></tr>`;
            return;
        }

        containers.forEach(c => {
            const tr = document.createElement('tr');
            tr.innerHTML = `
                <td>${c.pid}</td>
                <td><code>${c.command}</code></td>
                <td class="status-${c.status}">${c.status}</td>
            `;
            containersTable.appendChild(tr);
        });
    } catch (e) {
        console.error('Error fetching containers:', e);
    }
}

// Initial fetch and poll
fetchContainers();
setInterval(fetchContainers, 3000);

// Handle form submission
runForm.addEventListener('submit', async (e) => {
    e.preventDefault();
    
    const cmd = document.getElementById('cmd').value;
    const env = document.getElementById('env').value;
    const vol = document.getElementById('vol').value;
    const btn = runForm.querySelector('button');

    try {
        btn.textContent = 'Launching...';
        btn.disabled = true;

        const res = await fetch('/api/run', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ cmd, env, vol })
        });

        if (!res.ok) {
            const err = await res.text();
            throw new Error(err);
        }

        // Clear form
        document.getElementById('cmd').value = '';
        document.getElementById('env').value = '';
        document.getElementById('vol').value = '';
        
        // Refresh table immediately
        fetchContainers();
    } catch (e) {
        alert('Error launching container: ' + e.message);
    } finally {
        btn.textContent = 'Run Container';
        btn.disabled = false;
    }
});

// Handle shutdown
const shutdownBtn = document.getElementById('shutdownBtn');
if (shutdownBtn) {
    shutdownBtn.addEventListener('click', async () => {
        if (confirm('¿Estás seguro que deseas detener el servicio UI de Mini-Docker?')) {
            try {
                shutdownBtn.textContent = 'Deteniendo...';
                shutdownBtn.disabled = true;
                await fetch('/api/shutdown', { method: 'POST' });
                document.body.innerHTML = `
                    <div style="display:flex; justify-content:center; align-items:center; height:100vh; color:white; flex-direction: column;">
                        <h2>Servicio Detenido 🛑</h2>
                        <p style="color: #94a3b8; margin-top: 10px;">El proceso sudo en la consola ha finalizado.</p>
                        <p style="color: #94a3b8;">Puedes cerrar esta pestaña de forma segura.</p>
                    </div>`;
            } catch (e) {
                alert('Error al apagar: ' + e.message);
                shutdownBtn.textContent = 'Stop Service';
                shutdownBtn.disabled = false;
            }
        }
    });
}
