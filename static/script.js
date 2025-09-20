const api = {
	async sendLink(link) {
		const resp = await fetch('/', {
			method: 'POST',
			headers: { 'Content-Type': 'application/json' },
			body: JSON.stringify({ "link": link })
		});
		console.log('Response:', resp.status);
		return resp;
	},
	async getVideos() {
		const resp = await fetch('/api/videos');
		return resp;
	}
};

function displayMessage(message, type = 'info') {
	const container = document.getElementById('videos-container');
	const messageDiv = document.createElement('div');
	messageDiv.className = `message ${type}`;
	messageDiv.textContent = message;
	container.insertBefore(messageDiv, container.firstChild);

	// Remove message after 5 seconds
	setTimeout(() => {
		if (messageDiv.parentNode) {
			messageDiv.parentNode.removeChild(messageDiv);
		}
	}, 5000);
}

document.addEventListener("DOMContentLoaded", () => {
	console.log("Script loaded");

	// Handle form submission
	const form = document.getElementById('video-form');
	const linkInput = document.getElementById('link');

	form.addEventListener('submit', async (e) => {
		e.preventDefault();

		const link = linkInput.value.trim();
		if (!link) {
			displayMessage('Please enter a valid link', 'error');
			return;
		}

		try {
			displayMessage('Submitting link...', 'info');
			const response = await api.sendLink(link);

			if (response.ok) {
				displayMessage('Link submitted successfully!', 'success');
				linkInput.value = ''; // Clear the input
			} else {
				displayMessage('Error submitting link', 'error');
			}
		} catch (error) {
			console.error('Error:', error);
			displayMessage('Network error occurred', 'error');
		}
	});

	// Load videos on page load
	loadVideos();
});

async function loadVideos() {
	try {
		const response = await api.getVideos();
		if (response.ok) {
			const videos = await response.json();
			displayVideos(videos);
		}
	} catch (error) {
		console.error('Error loading videos:', error);
	}
}

function displayVideos(videos) {
	const container = document.getElementById('videos-container');

	console.log(videos);

	// Clear existing videos (but keep messages)
	const existingVideos = container.querySelectorAll('.video-item');
	existingVideos.forEach(item => item.remove());

	if (videos.length === 0) {
		const noVideos = document.createElement('div');
		noVideos.className = 'no-videos';
		noVideos.textContent = 'No videos available yet. Submit a link to get started!';
		container.appendChild(noVideos);
		return;
	}

	const videosList = document.createElement('div');
	videosList.className = 'videos-list';

	videos.forEach(video => {
		const videoItem = document.createElement('div');
		videoItem.className = 'video-item';

		const videoName = document.createElement('div');
		videoName.className = 'video-name';
		videoName.textContent = video.title;

		const videoInfo = document.createElement('div');
		videoInfo.className = 'video-info';
		videoInfo.textContent = `Size: ${formatFileSize(video.size)} | Modified: ${video.modified} | Uploader: ${video.uploader} | URL: ${video.url}`;

		const downloadLink = document.createElement('a');
		downloadLink.href = `/videos/${encodeURIComponent(video.filename)}`;
		downloadLink.textContent = 'Download';
		downloadLink.className = 'download-link';

		videoItem.appendChild(videoName);
		videoItem.appendChild(videoInfo);
		videoItem.appendChild(downloadLink);

		videosList.appendChild(videoItem);
	});

	container.appendChild(videosList);
}

function formatFileSize(bytes) {
	if (bytes === 0) return '0 Bytes';
	const k = 1024;
	const sizes = ['Bytes', 'KB', 'MB', 'GB'];
	const i = Math.floor(Math.log(bytes) / Math.log(k));
	return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
}
