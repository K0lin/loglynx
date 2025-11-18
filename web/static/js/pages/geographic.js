/**
 * Geographic Analytics Dashboard Page
 * With Leaflet.js interactive map
 */

let map, markers, markerClusterGroup, heatLayer;
let continentChart, topCountriesBarChart;
let allGeoData = {};
let currentLookupData = null;

// Load all geographic data
async function loadGeographicData() {
    try {
        // Load countries
        const countriesResult = await LogLynxAPI.getTopCountries(0); // 0 = all countries
        if (countriesResult.success) {
            allGeoData.countries = countriesResult.data;
            updateGeoKPIs(countriesResult.data);
            initCountryTable(countriesResult.data);
            updateContinentChart(countriesResult.data);
            updateTopCountriesBarChart(countriesResult.data);
            updateGeoInsights(countriesResult.data);
        }

        // Load IPs with geolocation
        const ipsResult = await LogLynxAPI.getTopIPs(100); // Get top 100 IPs
        if (ipsResult.success) {
            allGeoData.ips = ipsResult.data;
            initIPGeoTable(ipsResult.data);
            initCityTable(ipsResult.data);
            initializeMap(ipsResult.data, countriesResult.data);
        }

        // Load ASNs
        const asnResult = await LogLynxAPI.getTopASNs(30);
        if (asnResult.success) {
            allGeoData.asns = asnResult.data;
            updateASNGeoTable(asnResult.data);
        }

    } catch (error) {
        console.error('Error loading geographic data:', error);
        LogLynxUtils.showNotification('Failed to load geographic data', 'error');
    }
}

// Update geographic KPIs
function updateGeoKPIs(countriesData) {
    if (!countriesData || countriesData.length === 0) return;

    $('#totalCountries').text(countriesData.length);

    // Count unique cities
    const cities = new Set();
    if (allGeoData.ips) {
        allGeoData.ips.forEach(ip => {
            if (ip.city) cities.add(`${ip.city}-${ip.country}`);
        });
    }
    $('#totalCities').text(cities.size);

    // Count continents
    const continents = new Set();
    countriesData.forEach(country => {
        const continent = countryToContinentMap[country.country]?.name || 'Other';
        continents.add(continent);
    });
    $('#totalContinents').text(continents.size);

    // Top country
    if (countriesData.length > 0) {
        const top = countriesData[0];
        $('#topCountry').text(top.country_name || top.country);
        $('#topCountryHits').text(LogLynxUtils.formatNumber(top.hits) + ' hits');
    }
}

// Initialize Leaflet map
function initializeMap(ipsData, countriesData) {
    // Check if map already exists and remove it
    if (map) {
        map.remove();
        map = null;
    }

    // Initialize map
    map = L.map('worldMap').setView([20, 0], 2);

    // Add dark tile layer
    L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
        attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
        subdomains: 'abcd',
        maxZoom: 19
    }).addTo(map);

    // Initialize marker cluster group
    markerClusterGroup = L.markerClusterGroup({
        chunkedLoading: true,
        spiderfyOnMaxZoom: true,
        showCoverageOnHover: false,
        zoomToBoundsOnClick: true
    });

    markers = [];

    // Group IPs by location to aggregate data
    const locationMap = new Map();

    ipsData.forEach(ip => {
        if (ip.latitude && ip.longitude) {
            const key = `${ip.latitude},${ip.longitude}`;
            if (!locationMap.has(key)) {
                locationMap.set(key, {
                    lat: ip.latitude,
                    lon: ip.longitude,
                    country: ip.country,
                    city: ip.city,
                    ips: [],
                    totalHits: 0,
                    totalBandwidth: 0
                });
            }
            const loc = locationMap.get(key);
            loc.ips.push(ip.ip_address);
            loc.totalHits += ip.hits;
            loc.totalBandwidth += ip.bandwidth || 0;
        }
    });

    // Create markers
    locationMap.forEach((loc, key) => {
        const markerColor = getMarkerColor(loc.totalHits);
        const markerIcon = L.divIcon({
            className: 'custom-marker',
            html: `<div style="background-color: ${markerColor}; width: 12px; height: 12px; border-radius: 50%; border: 2px solid white;"></div>`,
            iconSize: [16, 16],
            iconAnchor: [8, 8]
        });

        const marker = L.marker([loc.lat, loc.lon], { icon: markerIcon });

        const popupContent = `
            <div style="min-width: 200px; color: #E8E8E8;">
                <strong style="font-size: 14px; color: #FFFFFF;">${loc.city || 'Unknown'}, ${loc.country || 'Unknown'}</strong>
                <hr style="margin: 8px 0; border-color: #444;">
                <div style="color: #E8E8E8;"><strong style="color: #FF6B35;">Coordinates:</strong> ${loc.lat.toFixed(4)}, ${loc.lon.toFixed(4)}</div>
                <div style="color: #E8E8E8;"><strong style="color: #FF6B35;">IPs:</strong> ${loc.ips.length}</div>
                <div style="color: #E8E8E8;"><strong style="color: #FF6B35;">Total Hits:</strong> ${LogLynxUtils.formatNumber(loc.totalHits)}</div>
                <div style="color: #E8E8E8;"><strong style="color: #FF6B35;">Bandwidth:</strong> ${LogLynxCharts.formatBytes(loc.totalBandwidth)}</div>
                <hr style="margin: 8px 0; border-color: #444;">
                <div style="max-height: 100px; overflow-y: auto; font-size: 11px; color: #E8E8E8;">
                    <strong style="color: #FF6B35;">IP Addresses:</strong><br>
                    ${loc.ips.slice(0, 5).map(ip => `<code style="background: #1A1A1D; color: #FFB800; padding: 2px 4px; border-radius: 3px;"><a href="/ip/${ip}">${ip}</a></code>`).join('<br>')}
                    ${loc.ips.length > 5 ? `<br><span style="color: #999;">...and ${loc.ips.length - 5} more</span>` : ''}
                </div>
            </div>
        `;

        marker.bindPopup(popupContent);
        markers.push(marker);
        markerClusterGroup.addLayer(marker);
    });

    map.addLayer(markerClusterGroup);

    $('#totalMarkers').text(markers.length);

    // Initialize heat layer (hidden by default)
    const heatPoints = [];
    locationMap.forEach(loc => {
        heatPoints.push([loc.lat, loc.lon, Math.log10(loc.totalHits + 1) / 4]);
    });

    heatLayer = L.heatLayer(heatPoints, {
        radius: 25,
        blur: 15,
        maxZoom: 10,
        gradient: {
            0.0: '#28a745',
            0.5: '#ffc107',
            0.7: '#F46319',
            1.0: '#dc3545'
        }
    });
}

// Get marker color based on hits
function getMarkerColor(hits) {
    if (hits > 10000) return '#dc3545'; // Red - Very High
    if (hits > 1000) return '#F46319';  // Orange - High
    if (hits > 100) return '#ffc107';   // Yellow - Medium
    return '#28a745';                    // Green - Low
}

// Map view controls
document.addEventListener('DOMContentLoaded', () => {
    $('#mapViewStreet').on('click', function() {
        if (!map) return;
        map.eachLayer(layer => {
            if (layer instanceof L.TileLayer) {
                map.removeLayer(layer);
            }
        });
        L.tileLayer('https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png', {
            attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors',
            maxZoom: 19
        }).addTo(map);
        $('.card-header button').removeClass('active');
        $(this).addClass('active');
    });

    $('#mapViewSatellite').on('click', function() {
        if (!map) return;
        map.eachLayer(layer => {
            if (layer instanceof L.TileLayer) {
                map.removeLayer(layer);
            }
        });
        L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
            attribution: '&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors &copy; <a href="https://carto.com/attributions">CARTO</a>',
            subdomains: 'abcd',
            maxZoom: 19
        }).addTo(map);
        $('.card-header button').removeClass('active');
        $(this).addClass('active');
    });

    $('#toggleHeatmap').on('click', function() {
        if (!map || !heatLayer) return;
        if (map.hasLayer(heatLayer)) {
            map.removeLayer(heatLayer);
            $(this).removeClass('active');
        } else {
            map.addLayer(heatLayer);
            $(this).addClass('active');
        }
    });

    $('#toggleClusters').on('click', function() {
        if (!map || !markerClusterGroup) return;
        if (map.hasLayer(markerClusterGroup)) {
            map.removeLayer(markerClusterGroup);
            $(this).removeClass('active');
        } else {
            map.addLayer(markerClusterGroup);
            $(this).addClass('active');
        }
    });
});

// Initialize continent chart
function initContinentChart() {
    continentChart = LogLynxCharts.createDoughnutChart('continentChart', {
        labels: [],
        datasets: [{
            data: [],
            backgroundColor: LogLynxCharts.colors.chartPalette
        }]
    });
}

// Update continent chart
function updateContinentChart(countriesData) {
    if (!continentChart) return;

    // Check for empty data and show empty state if needed
    if (LogLynxCharts.checkAndShowEmptyState(
        { datasets: [{ data: countriesData }] },
        'continentChart',
        'No continent data available'
    )) {
        // Clear chart data when empty
        continentChart.data.labels = [];
        continentChart.data.datasets[0].data = [];
        continentChart.update();
        return;
    }

    const continentData = {};
    countriesData.forEach(country => {
        const continent = countryToContinentMap[country.country]?.name || 'Other';
        continentData[continent] = (continentData[continent] || 0) + country.hits;
    });

    const labels = Object.keys(continentData);
    const values = Object.values(continentData);

    continentChart.data.labels = labels;
    continentChart.data.datasets[0].data = values;
    continentChart.update();
}

// Initialize top countries bar chart
function initTopCountriesBarChart() {
    topCountriesBarChart = LogLynxCharts.createHorizontalBarChart('topCountriesBarChart', {
        labels: [],
        datasets: [{
            label: 'Hits',
            data: [],
            backgroundColor: LogLynxCharts.colors.primaryLight + '80',
            borderColor: LogLynxCharts.colors.primary,
            borderWidth: 1
        }]
    });
}

// Update top countries bar chart
function updateTopCountriesBarChart(countriesData) {
    if (!topCountriesBarChart) return;

    // Check for empty data and show empty state if needed
    if (LogLynxCharts.checkAndShowEmptyState(
        { datasets: [{ data: countriesData }] },
        'topCountriesBarChart',
        'No countries data available'
    )) {
        // Clear chart data when empty
        topCountriesBarChart.data.labels = [];
        topCountriesBarChart.data.datasets[0].data = [];
        topCountriesBarChart.update();
        return;
    }

    const top10 = countriesData.slice(0, 10);
    const labels = top10.map(c => c.country_name || c.country);
    const values = top10.map(c => c.hits);

    topCountriesBarChart.data.labels = labels;
    topCountriesBarChart.data.datasets[0].data = values;
    topCountriesBarChart.update();
}

// Initialize country DataTable
function initCountryTable(countriesData) {
    if ($.fn.DataTable.isDataTable('#countryTable')) {
        $('#countryTable').DataTable().destroy();
    }

    // Check for empty data and show empty state if needed
    if (LogLynxUtils.checkAndShowEmptyState(
        countriesData,
        'countryTable',
        'datatable',
        'No country data available'
    )) {
        // Create empty DataTable with empty state
        $('#countryTable').DataTable({
            data: [],
            columns: [
                { data: null, render: (data, type, row, meta) => meta.row + 1 },
                { data: 'country', render: (d, type, row) => countryCodeToFlag(d, row.country_name) },
                { data: 'country_name', render: (d, type, row) => `<strong>${d || 'Unknown'}</strong>` },
                { data: 'country', render: (d) => `<code>${d || '-'}</code>` },
                { data: 'hits', render: (d) => LogLynxUtils.formatNumber(d) },
                { data: 'unique_visitors', render: (d) => LogLynxUtils.formatNumber(d || 0) },
                { data: 'bandwidth', render: (d) => LogLynxCharts.formatBytes(d || 0) },
                { data: null, render: (data) => '-' },
                { data: null, render: (data) => '<div style="width: 100%; height: 20px; background: #1f1f21; border-radius: 4px;"></div>' }
            ],
            order: [[4, 'desc']],
            pageLength: 20,
            autoWidth: false,
            responsive: true
        });
        return;
    }

    const total = countriesData.reduce((sum, c) => sum + c.hits, 0);

    $('#countryTable').DataTable({
        data: countriesData,
        columns: [
            {
                data: null,
                render: (data, type, row, meta) => meta.row + 1
            },
            {
                data: 'country',
                render: (d, type, row) => countryCodeToFlag(d, row.country_name)
            },
            {
                data: 'country_name',
                render: (d, type, row) => `<strong>${d || countryToContinentMap[row.country]?.name || 'Unknown'}</strong>` + `<small class='text-muted'>, ${countryToContinentMap[row.country]?.continent || 'Unknown'}</small>`
            },
            {
                data: 'country',
                render: (d) => `<code>${d || '-'}</code>`
            },
            {
                data: 'hits',
                render: (d) => LogLynxUtils.formatNumber(d)
            },
            {
                data: 'unique_visitors',
                render: (d) => LogLynxUtils.formatNumber(d || 0)
            },
            {
                data: 'bandwidth',
                render: (d) => LogLynxCharts.formatBytes(d || 0)
            },
            {
                data: null,
                render: (data) => {
                    // We don't have avg response time per country in current data
                    return '-';
                }
            },
            {
                data: null,
                render: (data) => {
                    const pct = total > 0 ? ((data.hits / total) * 100) : 0;
                    return `
                        <div style="width: 100%; height: 20px; background: #1f1f21; border-radius: 4px; overflow: hidden; position: relative;">
                            <div style="width: ${pct}%; height: 100%; background: ${LogLynxCharts.colors.primary}; transition: width 0.3s;"></div>
                            <span style="position: absolute; left: 50%; top: 50%; transform: translate(-50%, -50%); font-size: 0.7rem; color: #F3EFF3;">
                                ${pct.toFixed(2)}%
                            </span>
                        </div>
                    `;
                }
            }
        ],
        order: [[4, 'desc']],
        pageLength: 20,
        autoWidth: false,
        responsive: true,
        language: {
            emptyTable: 'No country data available'
        }
    });
}

// Initialize city DataTable
function initCityTable(ipsData) {
    if ($.fn.DataTable.isDataTable('#cityTable')) {
        $('#cityTable').DataTable().destroy();
    }

    // Aggregate by city
    const cityMap = new Map();
    ipsData.forEach(ip => {
        if (ip.city && ip.latitude && ip.longitude) {
            const key = `${ip.city}-${ip.country}`;
            if (!cityMap.has(key)) {
                cityMap.set(key, {
                    city: ip.city,
                    country: ip.country,
                    latitude: ip.latitude,
                    longitude: ip.longitude,
                    hits: 0,
                    bandwidth: 0
                });
            }
            const city = cityMap.get(key);
            city.hits += ip.hits;
            city.bandwidth += ip.bandwidth || 0;
        }
    });

    const citiesData = Array.from(cityMap.values())
        .sort((a, b) => b.hits - a.hits)
        .slice(0, 50);

    // Check for empty data and show empty state if needed
    if (LogLynxUtils.checkAndShowEmptyState(
        citiesData,
        'cityTable',
        'datatable',
        'No city data available'
    )) {
        // Create empty DataTable with empty state
        $('#cityTable').DataTable({
            data: [],
            columns: [
                { data: null, render: (data, type, row, meta) => meta.row + 1 },
                { data: 'city', render: (d) => `<strong>${d}</strong>` },
                { data: 'country', render: (d) => d || '-' },
                { data: 'latitude', render: (d) => d ? d.toFixed(4) : '-' },
                { data: 'longitude', render: (d) => d ? d.toFixed(4) : '-' },
                { data: 'hits', render: (d) => LogLynxUtils.formatNumber(d) },
                { data: 'bandwidth', render: (d) => LogLynxCharts.formatBytes(d) },
                { data: null, render: (data) => '<button class="btn btn-sm btn-secondary" disabled><i class="fas fa-map-marker-alt"></i> View</button>' }
            ],
            order: [[5, 'desc']],
            pageLength: 20,
            autoWidth: false,
            responsive: true
        });
        return;
    }

    $('#cityTable').DataTable({
        data: citiesData,
        columns: [
            {
                data: null,
                render: (data, type, row, meta) => meta.row + 1
            },
            {
                data: 'city',
                render: (d) => `<strong>${d}</strong>`
            },
            {
                data: 'country',
                render: (d) => d || '-'
            },
            {
                data: 'latitude',
                render: (d) => d.toFixed(4)
            },
            {
                data: 'longitude',
                render: (d) => d.toFixed(4)
            },
            {
                data: 'hits',
                render: (d) => LogLynxUtils.formatNumber(d)
            },
            {
                data: 'bandwidth',
                render: (d) => LogLynxCharts.formatBytes(d)
            },
            {
                data: null,
                render: (data) => {
                    return `<button class="btn btn-sm btn-primary" onclick="flyToLocation(${data.latitude}, ${data.longitude}, '${data.city}')">
                        <i class="fas fa-map-marker-alt"></i> View
                    </button>`;
                }
            }
        ],
        order: [[5, 'desc']],
        pageLength: 20,
        autoWidth: false,
        responsive: true,
        language: {
            emptyTable: 'No city data available'
        }
    });
}

// Initialize IP geolocation DataTable
function initIPGeoTable(ipsData) {
    if ($.fn.DataTable.isDataTable('#ipGeoTable')) {
        $('#ipGeoTable').DataTable().destroy();
    }

    // Check for empty data and show empty state if needed
    if (LogLynxUtils.checkAndShowEmptyState(
        ipsData,
        'ipGeoTable',
        'datatable',
        'No IP geolocation data available'
    )) {
        // Create empty DataTable with empty state
        $('#ipGeoTable').DataTable({
            data: [],
            columns: [
                { data: 'ip_address', render: (d) => `<a href="/ip/${d}" class="ip-link"><code>${d}</code></a>` },
                { data: 'country', render: (d) => d || '-' },
                { data: 'city', render: (d) => d || '-' },
                { data: null, render: (data) => '-' },
                { data: null, render: (data) => '-' },
                { data: null, render: (data) => '-' },
                { data: 'hits', render: (d) => LogLynxUtils.formatNumber(d) },
                { data: null, render: (data) => '<button class="btn btn-sm btn-secondary" disabled><i class="fas fa-map-marker-alt"></i> Map</button>' }
            ],
            order: [[6, 'desc']],
            pageLength: 20,
            autoWidth: false,
            responsive: true
        });
        return;
    }

    $('#ipGeoTable').DataTable({
        data: ipsData,
        columns: [
            {
                data: 'ip_address',
                render: (d) => `<a href="/ip/${d}" class="ip-link"><code>${d}</code></a>`
            },
            {
                data: 'country',
                render: (d) => d || '-'
            },
            {
                data: 'city',
                render: (d) => d || '-'
            },
            {
                data: null,
                render: (data) => {
                    if (data.latitude && data.longitude) {
                        return `${data.latitude.toFixed(4)}, ${data.longitude.toFixed(4)}`;
                    }
                    return '-';
                }
            },
            {
                data: null,
                render: (data) => {
                    // ASN data not in IP data, would need separate query
                    return '-';
                }
            },
            {
                data: null,
                render: (data) => {
                    return '-';
                }
            },
            {
                data: 'hits',
                render: (d) => LogLynxUtils.formatNumber(d)
            },
            {
                data: null,
                render: (data) => {
                    if (data.latitude && data.longitude) {
                        return `<button class="btn btn-sm btn-primary" onclick="flyToLocation(${data.latitude}, ${data.longitude}, '${data.ip_address}')">
                            <i class="fas fa-map-marker-alt"></i> Map
                        </button>`;
                    }
                    return '-';
                }
            }
        ],
        order: [[6, 'desc']],
        pageLength: 20,
        autoWidth: false,
        responsive: true,
        language: {
            emptyTable: 'No IP geolocation data available'
        }
    });
}

// Update ASN geographic table
function updateASNGeoTable(asnData) {
    let html = '';

    if (!asnData || asnData.length === 0) {
        html = '<tr><td colspan="7" class="text-center text-muted">No ASN data available</td></tr>';
    } else {
        asnData.forEach((asn, index) => {
            html += `
                <tr>
                    <td>${index + 1}</td>
                    <td><strong>AS${asn.asn}</strong></td>
                    <td>${LogLynxUtils.truncate(asn.asn_org || 'Unknown', 40)}</td>
                    <td>${asn.country || '-'}</td>
                    <td>1</td>
                    <td>${LogLynxUtils.formatNumber(asn.hits)}</td>
                    <td>${LogLynxCharts.formatBytes(asn.bandwidth || 0)}</td>
                </tr>
            `;
        });
    }

    $('#asnGeoTable').html(html);
}

// Update geographic insights
function updateGeoInsights(countriesData) {
    if (!countriesData || countriesData.length === 0) return;

    // Countries with traffic
    const totalCountries = 195; // Approximate total countries in world
    const countriesWithTraffic = countriesData.length;
    const countriesPct = ((countriesWithTraffic / totalCountries) * 100).toFixed(1);

    $('#countriesWithTraffic').text(countriesWithTraffic);
    $('#countriesPercent').text(`${countriesPct}% of world`);
    $('#countriesBar').css('width', countriesPct + '%');

    // Cities with traffic
    const cities = new Set();
    if (allGeoData.ips) {
        allGeoData.ips.forEach(ip => {
            if (ip.city) cities.add(`${ip.city}-${ip.country}`);
        });
    }
    $('#citiesWithTraffic').text(cities.size);

    // Geographic spread
    let spread = 'Regional';
    if (countriesWithTraffic > 50) spread = 'Global';
    else if (countriesWithTraffic > 20) spread = 'Continental';
    else if (countriesWithTraffic > 5) spread = 'Multi-Regional';
    $('#geoSpread').text(spread);

    // Highest traffic country
    if (countriesData.length > 0) {
        const top = countriesData[0];
        $('#highestTrafficCountry').text(top.country_name || top.country);
        $('#highestTrafficHits').text(LogLynxUtils.formatNumber(top.hits) + ' hits');
    }

    // New countries/cities (mock data - would need historical comparison)
    $('#newCountries').text('0');
    $('#newCities').text('0');
    $('#trustedRegions').text(Math.min(countriesWithTraffic, 10));
}

// Fly to location on map
window.flyToLocation = function(lat, lon, label) {
    if (map) {
        map.flyTo([lat, lon], 8, {
            duration: 2
        });
        LogLynxUtils.showNotification(`Flying to ${label}`, 'info', 2000);
    }
};

// IP lookup functionality
window.lookupIPLocation = async function() {
    const ip = $('#lookupIP').val().trim();
    if (!ip) {
        LogLynxUtils.showNotification('Please enter an IP address', 'warning');
        return;
    }

    // Find IP in our data
    const ipData = allGeoData.ips ? allGeoData.ips.find(i => i.ip_address === ip) : null;

    if (ipData) {
        currentLookupData = ipData;
        $('#lookupResultIP').text(ip);
        $('#lookupCountry').text(ipData.country || 'Unknown');
        $('#lookupCity').text(ipData.city || 'Unknown');
        $('#lookupCoords').text(ipData.latitude && ipData.longitude ?
            `${ipData.latitude.toFixed(4)}, ${ipData.longitude.toFixed(4)}` : 'Unknown');
        $('#lookupASN').text('-');
        $('#lookupOrg').text('-');
        $('#lookupResult').show();
    } else {
        LogLynxUtils.showNotification('IP address not found in current data', 'warning');
        $('#lookupResult').hide();
    }
};

// Show lookup result on map
window.showOnMap = function() {
    if (currentLookupData && currentLookupData.latitude && currentLookupData.longitude) {
        flyToLocation(currentLookupData.latitude, currentLookupData.longitude, currentLookupData.ip_address);
        LogLynxUtils.scrollToElement('worldMap');
    }
};

// Export functions
function exportGeoData() {
    const report = {
        countries: allGeoData.countries,
        ips: allGeoData.ips,
        asns: allGeoData.asns
    };

    const blob = new Blob([JSON.stringify(report, null, 2)], { type: 'application/json' });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `geographic-report-${new Date().toISOString().split('T')[0]}.json`;
    link.click();

    LogLynxUtils.showNotification('Geographic data exported', 'success', 3000);
}

function exportCountryData() {
    const table = $('#countryTable').DataTable();
    const data = table.rows().data().toArray();
    LogLynxUtils.exportAsCSV(data, 'country-analysis.csv');
}

function exportCityData() {
    const table = $('#cityTable').DataTable();
    const data = table.rows().data().toArray();
    LogLynxUtils.exportAsCSV(data, 'city-analysis.csv');
}

function exportIPGeoData() {
    const table = $('#ipGeoTable').DataTable();
    const data = table.rows().data().toArray();
    LogLynxUtils.exportAsCSV(data, 'ip-geolocation.csv');
}

function exportASNGeoData() {
    if (allGeoData.asns) {
        LogLynxUtils.exportAsCSV(allGeoData.asns, 'asn-geographic.csv');
    }
}

// Initialize service filter with reload callback
function initServiceFilterWithReload() {
    LogLynxUtils.initServiceFilter(() => {
        loadGeographicData();
    });
}

// Initialize page
// Initialize hide my traffic filter with reload callback
function initHideTrafficFilterWithReload() {
    LogLynxUtils.initHideMyTrafficFilter(() => {
        loadGeographicData();
    });
}

document.addEventListener('DOMContentLoaded', () => {
    // Initialize charts
    initContinentChart();
    initTopCountriesBarChart();

    // Initialize controls
    initServiceFilterWithReload();
    initHideTrafficFilterWithReload();

    // Initialize refresh controls (will do initial data load automatically)
    LogLynxUtils.initRefreshControls(loadGeographicData, 60); // 60 seconds for map data
});
