const api = {
	async sendLink(link, onProgress = null) {
		try {
			const controller = new AbortController();
			const timeoutId = setTimeout(() => controller.abort(), 300000); // 5 minute timeout
			
			const resp = await fetch('/', {
				method: 'POST',
				headers: { 'Content-Type': 'application/json' },
				body: JSON.stringify({ "link": link }),
				signal: controller.signal
			});
			
			clearTimeout(timeoutId);
			console.log('Response:', resp.status);
			
			// Parse response body for detailed error information
			const responseData = await this.parseResponse(resp);
			
			return {
				ok: resp.ok,
				status: resp.status,
				statusText: resp.statusText,
				data: responseData,
				response: resp
			};
		} catch (error) {
			if (error.name === 'AbortError') {
				throw new Error('Request timed out. The download may be taking longer than expected.');
			}
			throw error;
		}
	},
	
	async getVideos() {
		try {
			const resp = await fetch('/api/videos');
			const responseData = await this.parseResponse(resp);
			
			return {
				ok: resp.ok,
				status: resp.status,
				statusText: resp.statusText,
				data: responseData,
				response: resp
			};
		} catch (error) {
			throw new Error(`Failed to load videos: ${error.message}`);
		}
	},
	
	async parseResponse(response) {
		const contentType = response.headers.get('content-type');
		
		if (contentType && contentType.includes('application/json')) {
			try {
				return await response.json();
			} catch (error) {
				throw new Error('Invalid JSON response from server');
			}
		} else {
			const text = await response.text();
			return { message: text || response.statusText };
		}
	},
	
	getErrorMessage(status, data) {
		// Map HTTP status codes to user-friendly messages
		const statusMessages = {
			400: 'Invalid request. Please check your link format.',
			401: 'Authentication required.',
			403: 'Access denied.',
			404: 'Service not found.',
			408: 'Request timed out. Please try again.',
			429: 'Too many requests. Please wait before trying again.',
			500: 'Server error. Please try again later.',
			502: 'Service temporarily unavailable.',
			503: 'Service temporarily unavailable.',
			504: 'Request timed out. The server took too long to respond.'
		};
		
		// Use server-provided error message if available
		if (data && data.error) {
			return data.error.toString();
		}
		
		if (data && data.message) {
			return data.message;
		}
		
		// Fall back to status-based message
		return statusMessages[status] || `Request failed with status ${status}`;
	}
};


function renderVideoCard(video) {
	const videoItem = document.createElement('div');
	videoItem.className = 'video-item';

	const videoName = document.createElement('div');
	videoName.className = 'video-name';
	videoName.textContent = video.title;

	const videoInfo = document.createElement('div');
	videoInfo.className = 'video-info';
	videoInfo.innerHTML = `Size: ${formatFileSize(video.size)} | Modified: ${video.modified} | Views: ${formatViewCount(video.views)} | Uploader: ${video.uploader} | <a href="${video.url}" id="video-url"></a>`;
	videoInfo.querySelector("#video-url").appendChild(newMaterialIcon('link'));

	// Extra info section (visible depending on screen size)
	const videoExtraInfo = document.createElement('div');
	videoExtraInfo.className = 'video-extra-info';
	videoExtraInfo.style.display = 'none';

	const vidDescHead = document.createElement('span');
	vidDescHead.innerHTML = 'Description:<br>';

	const vidDesc = document.createElement('p');
	vidDesc.innerHTML = `${formatDescription(video.description)}`;

	videoExtraInfo.appendChild(vidDescHead);
	videoExtraInfo.appendChild(vidDesc);

	const showIcon = newMaterialIcon('visibility');
	const hideIcon = newMaterialIcon('visibility_off');

	const toggleButton = document.createElement('button');
	toggleButton.className = 'toggle-description-button';
	toggleButton.appendChild(showIcon);

	toggleButton.addEventListener('click', () => {
		const isVisible = videoExtraInfo.style.display === 'block';
		videoExtraInfo.style.display = isVisible ? 'none' : 'block';

		toggleButton.innerHTML = ''
		toggleButton.appendChild(isVisible ? showIcon : hideIcon);
	});

	const downloadLink = document.createElement('a');
	downloadLink.href = `/videos/${encodeURIComponent(video.filename)}`;
	downloadLink.textContent = 'Download';
	downloadLink.className = 'download-link';

	videoItem.appendChild(videoName);
	videoItem.appendChild(videoInfo);
	videoItem.appendChild(toggleButton);
	videoItem.appendChild(videoExtraInfo);
	videoItem.appendChild(downloadLink);

	return videoItem;
}

function displayMessage(message, type = 'info', options = {}) {
	const container = document.getElementById('videos-container');
	const messageDiv = document.createElement('div');
	messageDiv.className = `message ${type}`;
	
	// Create message content with optional action buttons
	const messageContent = document.createElement('div');
	messageContent.className = 'message-content';
	
	const messageText = document.createElement('span');
	messageText.className = 'message-text';
	messageText.textContent = message;
	messageContent.appendChild(messageText);
	
	// Add retry button for errors if callback provided
	if (type === 'error' && options.onRetry) {
		const retryButton = document.createElement('button');
		retryButton.className = 'message-retry-btn';
		retryButton.textContent = 'Retry';
		retryButton.onclick = options.onRetry;
		messageContent.appendChild(retryButton);
	}
	
	// Add close button for persistent messages
	if (options.persistent) {
		const closeButton = document.createElement('button');
		closeButton.className = 'message-close-btn';
		closeButton.innerHTML = '×';
		closeButton.onclick = () => removeMessage(messageDiv);
		messageContent.appendChild(closeButton);
	}
	
	messageDiv.appendChild(messageContent);
	
	// Add progress bar for loading messages
	if (type === 'loading' || options.showProgress) {
		const progressBar = document.createElement('div');
		progressBar.className = 'message-progress';
		const progressFill = document.createElement('div');
		progressFill.className = 'message-progress-fill';
		progressBar.appendChild(progressFill);
		messageDiv.appendChild(progressBar);
		
		// Store reference for updating progress
		messageDiv.progressFill = progressFill;
	}
	
	container.insertBefore(messageDiv, container.firstChild);
	
	// Auto-remove message unless persistent
	if (!options.persistent) {
		const timeout = options.timeout || (type === 'error' ? 8000 : 5000);
		setTimeout(() => removeMessage(messageDiv), timeout);
	}
	
	return messageDiv;
}

function removeMessage(messageDiv) {
	if (messageDiv && messageDiv.parentNode) {
		messageDiv.style.opacity = '0';
		messageDiv.style.transform = 'translateY(-10px)';
		setTimeout(() => {
			if (messageDiv.parentNode) {
				messageDiv.parentNode.removeChild(messageDiv);
			}
		}, 300);
	}
}

function updateMessageProgress(messageDiv, progress) {
	if (messageDiv && messageDiv.progressFill) {
		messageDiv.progressFill.style.width = `${Math.min(100, Math.max(0, progress))}%`;
	}
}

function showLoadingState(isLoading) {
	const form = document.getElementById('video-form');
	const submitButton = form.querySelector('input[type="submit"]');
	const linkInput = document.getElementById('link');
	
	if (isLoading) {
		submitButton.disabled = true;
		submitButton.value = 'Processing...';
		submitButton.classList.add('loading');
		linkInput.disabled = true;
		form.classList.add('form-loading');
	} else {
		submitButton.disabled = false;
		submitButton.value = 'Download';
		submitButton.classList.remove('loading');
		linkInput.disabled = false;
		form.classList.remove('form-loading');
	}
}

class RetryManager {
	constructor(maxRetries = 3, baseDelay = 1000) {
		this.maxRetries = maxRetries;
		this.baseDelay = baseDelay;
		this.attempts = new Map();
	}
	
	async execute(key, operation, onRetry = null) {
		const attemptCount = this.attempts.get(key) || 0;
		
		try {
			const result = await operation();
			this.attempts.delete(key); // Reset on success
			return result;
		} catch (error) {
			if (attemptCount < this.maxRetries) {
				const nextAttempt = attemptCount + 1;
				this.attempts.set(key, nextAttempt);
				
				const delay = this.baseDelay * Math.pow(2, attemptCount); // Exponential backoff
				
				if (onRetry) {
					onRetry(nextAttempt, this.maxRetries, delay);
				}
				
				await new Promise(resolve => setTimeout(resolve, delay));
				return this.execute(key, operation, onRetry);
			} else {
				this.attempts.delete(key);
				throw new Error(`Operation failed after ${this.maxRetries} attempts: ${error.message}`);
			}
		}
	}
	
	reset(key) {
		this.attempts.delete(key);
	}
}

const retryManager = new RetryManager();

document.addEventListener("DOMContentLoaded", () => {
	console.log("Script loaded");

	// Handle form submission
	const form = document.getElementById('video-form');
	const linkInput = document.getElementById('link');

	form.addEventListener('submit', async (e) => {
		e.preventDefault();
		await handleVideoSubmission();
	});

	// Load videos on page load
	loadVideos();
});

async function handleVideoSubmission() {
	const linkInput = document.getElementById('link');
	const link = linkInput.value.trim();
	
	// Input validation
	if (!link) {
		displayMessage('Please enter a valid link', 'error');
		return;
	}
	
	// Basic URL validation
	try {
		new URL(link);
	} catch {
		displayMessage('Please enter a valid URL (e.g., https://youtube.com/watch?v=...)', 'error');
		return;
	}
	
	// Show loading state
	showLoadingState(true);
	
	// Create progress message
	const progressMessage = displayMessage('Processing your request...', 'loading', { 
		showProgress: true,
		persistent: true 
	});
	
	// Simulate progress updates (since we don't have real progress from backend)
	let progress = 0;
	const progressInterval = setInterval(() => {
		progress += Math.random() * 15;
		if (progress > 90) progress = 90; // Don't complete until we get response
		updateMessageProgress(progressMessage, progress);
	}, 500);
	
	try {
		const response = await retryManager.execute(
			`submit-${link}`,
			() => api.sendLink(link),
			(attempt, maxAttempts, delay) => {
				removeMessage(progressMessage);
				displayMessage(
					`Attempt ${attempt}/${maxAttempts} failed. Retrying in ${Math.round(delay/1000)} seconds...`, 
					'warning',
					{ timeout: delay }
				);
			}
		);
		
		// Complete progress
		clearInterval(progressInterval);
		updateMessageProgress(progressMessage, 100);
		
		setTimeout(() => {
			removeMessage(progressMessage);
			
			if (response.ok) {
				// Check if there was an error in the response data
				if (response.data && response.data.error && response.data.error !== null) {
					const errorMsg = api.getErrorMessage(response.status, response.data);
					displayMessage(`Download failed: ${errorMsg}`, 'error', {
						persistent: true,
						onRetry: () => {
							retryManager.reset(`submit-${link}`);
							handleVideoSubmission();
						}
					});
				} else {
					displayMessage('Video download started successfully!', 'success');
					linkInput.value = ''; // Clear the input
					
					// Refresh video list after a delay to show new video
					setTimeout(() => {
						loadVideos();
					}, 2000);
				}
			} else {
				const errorMsg = api.getErrorMessage(response.status, response.data);
				displayMessage(`Error: ${errorMsg}`, 'error', {
					persistent: true,
					onRetry: () => {
						retryManager.reset(`submit-${link}`);
						handleVideoSubmission();
					}
				});
			}
		}, 500);
		
	} catch (error) {
		clearInterval(progressInterval);
		removeMessage(progressMessage);
		
		console.error('Submission error:', error);
		
		// Determine error type and message
		let errorMessage = 'An unexpected error occurred';
		let showRetry = true;
		
		if (error.message.includes('timeout')) {
			errorMessage = 'Request timed out. The video might be large or the server is busy.';
		} else if (error.message.includes('network') || error.message.includes('fetch')) {
			errorMessage = 'Network error. Please check your connection.';
		} else if (error.message.includes('attempts')) {
			errorMessage = error.message;
			showRetry = false; // Already tried multiple times
		} else {
			errorMessage = error.message;
		}
		
		displayMessage(errorMessage, 'error', {
			persistent: true,
			onRetry: showRetry ? () => {
				retryManager.reset(`submit-${link}`);
				handleVideoSubmission();
			} : null
		});
	} finally {
		showLoadingState(false);
	}
}

async function loadVideos() {
	try {
		const response = await retryManager.execute(
			'load-videos',
			() => api.getVideos(),
			(attempt, maxAttempts) => {
				displayMessage(
					`Failed to load videos. Retrying... (${attempt}/${maxAttempts})`, 
					'warning',
					{ timeout: 2000 }
				);
			}
		);
		
		if (response.ok) {
			displayVideos(response.data);
		} else {
			const errorMsg = api.getErrorMessage(response.status, response.data);
			displayMessage(`Failed to load videos: ${errorMsg}`, 'error', {
				onRetry: () => {
					retryManager.reset('load-videos');
					loadVideos();
				}
			});
		}
	} catch (error) {
		console.error('Error loading videos:', error);
		displayMessage(`Unable to load videos: ${error.message}`, 'error', {
			onRetry: () => {
				retryManager.reset('load-videos');
				loadVideos();
			}
		});
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
		videoItem = renderVideoCard(video)
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
function formatViewCount(number) {
	if (number < 0) return '0';
	const k = 1000;
	const sizes = ['', 'K', 'M']
	const i = Math.floor(Math.log(number) / Math.log(k));
	return parseFloat((number) / Math.pow(k, i)).toFixed(2) + sizes[i];
}

function formatDescription(raw) {
	const withLinks = raw.replace(
		/(https?:\/\/[^\s<]+)/g,
		'<a href="$1" target="_blank" rel="noopener noreferrer">$1</a>'
	);

	const desc = withLinks.replace(/\n/g, "<br>")
	return desc;
}
function newMaterialIcon(name) {
	const icon = document.createElement('i');
	icon.className = 'material-icons';
	icon.textContent = name;
	icon.style.color = 'inherit' // Can override after
	return icon;
}


